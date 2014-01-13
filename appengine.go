// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Package appengine provides basic functionality for Google App Engine.
//
// For more information on how to write Go apps for Google App Engine, see:
// https://developers.google.com/appengine/docs/go/
package appengine

import (
	"net/http"

	"code.google.com/p/goprotobuf/proto"
)

// Context represents the context of an in-flight HTTP request.
type Context interface {
	// Debugf formats its arguments according to the format, analogous to fmt.Printf,
	// and records the text as a log message at Debug level.
	Debugf(format string, args ...interface{})

	// Infof is like Debugf, but at Info level.
	Infof(format string, args ...interface{})

	// Warningf is like Debugf, but at Warning level.
	Warningf(format string, args ...interface{})

	// Errorf is like Debugf, but at Error level.
	Errorf(format string, args ...interface{})

	// Criticalf is like Debugf, but at Critical level.
	Criticalf(format string, args ...interface{})

	// The remaining methods are for internal use only.
	// Developer-facing APIs wrap these methods to provide a more friendly API.

	// Internal use only.
	// TODO(dsymonds): Use *appengine_internal.CallOptions for opts.
	Call(service, method string, in, out proto.Message, opts interface{}) error
	// Internal use only. Use AppID instead.
	FullyQualifiedAppID() string
	// Internal use only.
	Request() interface{}
}
