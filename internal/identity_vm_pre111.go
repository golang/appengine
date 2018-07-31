// Copyright 2018 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// +build !go1.11

package internal

import (
	"net/http"

	netcontext "golang.org/x/net/context"
)

const (
	hDefaultVersionHostname = "X-AppEngine-Default-Version-Hostname"
	hRequestLogId           = "X-AppEngine-Request-Log-Id"
	hDatacenter             = "X-AppEngine-Datacenter"
)

func ctxHeaders(ctx netcontext.Context) http.Header {
	c := fromContext(ctx)
	if c == nil {
		return nil
	}
	return c.Request().Header
}

func DefaultVersionHostname(ctx netcontext.Context) string {
	return ctxHeaders(ctx).Get(hDefaultVersionHostname)
}

func RequestID(ctx netcontext.Context) string {
	return ctxHeaders(ctx).Get(hRequestLogId)
}

func Datacenter(ctx netcontext.Context) string {
	return ctxHeaders(ctx).Get(hDatacenter)
}
