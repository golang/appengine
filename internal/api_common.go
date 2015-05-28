package internal

import (
	"github.com/golang/protobuf/proto"
	netcontext "golang.org/x/net/context"
)

type callOverrideFunc func(ctx netcontext.Context, service, method string, in, out proto.Message) error

var callOverrideKey = "holds []callOverrideFunc"

func WithCallOverride(ctx netcontext.Context, f callOverrideFunc) netcontext.Context {
	// We avoid appending to any existing call override
	// so we don't risk overwriting a popped stack below.
	var cofs []callOverrideFunc
	if uf, ok := ctx.Value(&callOverrideKey).([]callOverrideFunc); ok {
		cofs = append(cofs, uf...)
	}
	cofs = append(cofs, f)
	return netcontext.WithValue(ctx, &callOverrideKey, cofs)
}

func callOverrideFromContext(ctx netcontext.Context) (callOverrideFunc, netcontext.Context, bool) {
	cofs, _ := ctx.Value(&callOverrideKey).([]callOverrideFunc)
	if len(cofs) == 0 {
		return nil, nil, false
	}
	// We found a list of overrides; grab the last, and reconstitute a
	// context that will hide it.
	f := cofs[len(cofs)-1]
	ctx = netcontext.WithValue(ctx, &callOverrideKey, cofs[:len(cofs)-1])
	return f, ctx, true
}

type logOverrideFunc func(level int64, format string, args ...interface{})

var logOverrideKey = "holds a logOverrideFunc"

func WithLogOverride(ctx netcontext.Context, f logOverrideFunc) netcontext.Context {
	return netcontext.WithValue(ctx, &logOverrideKey, f)
}

var appIDOverrideKey = "holds a string, being the full app ID"

func WithAppIDOverride(ctx netcontext.Context, appID string) netcontext.Context {
	return netcontext.WithValue(ctx, &appIDOverrideKey, appID)
}

var namespaceKey = "holds the namespace string"

func WithNamespace(ctx netcontext.Context, ns string) netcontext.Context {
	return netcontext.WithValue(ctx, &namespaceKey, ns)
}

func NamespaceFromContext(ctx netcontext.Context) string {
	// If there's no namespace, return the empty string.
	ns, _ := ctx.Value(&namespaceKey).(string)
	return ns
}

// FullyQualifiedAppID returns the fully-qualified application ID.
// This may contain a partition prefix (e.g. "s~" for High Replication apps),
// or a domain prefix (e.g. "example.com:").
func FullyQualifiedAppID(ctx netcontext.Context) string {
	if id, ok := ctx.Value(&appIDOverrideKey).(string); ok {
		return id
	}
	return fullyQualifiedAppID(ctx)
}

func Logf(ctx netcontext.Context, level int64, format string, args ...interface{}) {
	if f, ok := ctx.Value(&logOverrideKey).(logOverrideFunc); ok {
		f(level, format, args...)
		return
	}
	logf(fromContext(ctx), level, format, args...)
}
