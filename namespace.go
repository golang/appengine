// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package appengine

import (
	"fmt"
	"regexp"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"google.golang.org/appengine/internal"
	basepb "google.golang.org/appengine/internal/base"
)

// Namespace returns a replacement context that operates within the given namespace.
func Namespace(c context.Context, namespace string) (context.Context, error) {
	if !validNamespace.MatchString(namespace) {
		return nil, fmt.Errorf("appengine: namespace %q does not match /%s/", namespace, validNamespace)
	}
	n := &namespacedContext{
		ctx:       c,
		namespace: namespace,
	}
	return internal.WithCallOverride(c, n.call), nil
}

// validNamespace matches valid namespace names.
var validNamespace = regexp.MustCompile(`^[0-9A-Za-z._-]{0,100}$`)

// namespacedContext wraps a Context to support namespaces.
type namespacedContext struct {
	ctx       context.Context
	namespace string
}

func (n *namespacedContext) call(_ context.Context, service, method string, in, out proto.Message, opts *internal.CallOptions) error {
	// Apply any namespace mods.
	if mod, ok := internal.NamespaceMods[service]; ok {
		mod(in, n.namespace)
	}
	if service == "__go__" && method == "GetNamespace" {
		out.(*basepb.StringProto).Value = proto.String(n.namespace)
		return nil
	}

	return internal.Call(n.ctx, service, method, in, out, opts)
}
