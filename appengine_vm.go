// Copyright 2015 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// +build !appengine

package appengine

import (
	"golang.org/x/net/context"

	"google.golang.org/appengine/internal"
)

// BackgroundContext returns a context not associated with a request.
func BackgroundContext() context.Context {
	return internal.BackgroundContext()
}
