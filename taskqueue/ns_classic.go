// +build appengine

package taskqueue

import (
	"golang.org/x/net/context"

	"google.golang.org/appengine/internal"
	basepb "google.golang.org/appengine/internal/base"
)

func getDefaultNamespace(ctx context.Context) string {
	c := internal.ClassicContextFromContext(ctx)
	s := &basepb.StringProto{}
	c.Call("__go__", "GetDefaultNamespace", &basepb.VoidProto{}, s, nil)
	return s.GetValue()
}
