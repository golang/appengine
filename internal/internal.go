// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Package internal provides support for package appengine.
//
// Programs should not use this package directly. Its API is not stable.
// Use packages appengine and appengine/* instead.
package internal

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"code.google.com/p/goprotobuf/proto"
)

type CallOptions struct {
	Timeout time.Duration // if non-zero, overrides RPC default
}

// errorCodeMaps is a map of service name to the error code map for the service.
var errorCodeMaps = make(map[string]map[int32]string)

// RegisterErrorCodeMap is called from API implementations to register their
// error code map. This should only be called from init functions.
func RegisterErrorCodeMap(service string, m map[int32]string) {
	errorCodeMaps[service] = m
}

type timeoutCodeKey struct {
	service string
	code    int32
}

// timeoutCodes is the set of service+code pairs that represent timeouts.
var timeoutCodes = make(map[timeoutCodeKey]bool)

func RegisterTimeoutErrorCode(service string, code int32) {
	timeoutCodes[timeoutCodeKey{service, code}] = true
}

// APIError is the type returned by appengine.Context's Call method
// when an API call fails in an API-specific way. This may be, for instance,
// a taskqueue API call failing with TaskQueueServiceError::UNKNOWN_QUEUE.
type APIError struct {
	Service string
	Detail  string
	Code    int32 // API-specific error code
}

func (e *APIError) Error() string {
	if e.Code == 0 {
		if e.Detail == "" {
			return "APIError <empty>"
		}
		return e.Detail
	}
	s := fmt.Sprintf("API error %d", e.Code)
	if m, ok := errorCodeMaps[e.Service]; ok {
		s += " (" + e.Service + ": " + m[e.Code] + ")"
	} else {
		// Shouldn't happen, but provide a bit more detail if it does.
		s = e.Service + " " + s
	}
	if e.Detail != "" {
		s += ": " + e.Detail
	}
	return s
}

func (e *APIError) IsTimeout() bool {
	return timeoutCodes[timeoutCodeKey{e.Service, e.Code}]
}

// CallError is the type returned by appengine.Context's Call method when an
// API call fails in a generic way, such as APIResponse::CAPABILITY_DISABLED.
type CallError struct {
	Detail string
	Code   int32
	// TODO: Remove this if we get a distinguishable error code.
	Timeout bool
}

func (e *CallError) Error() string {
	var msg string
	switch e.Code {
	case 0: // OK
		return e.Detail
	case 4: // OVER_QUOTA
		msg = "Over quota"
	case 6: // CAPABILITY_DISABLED
		msg = "Capability disabled"
	case 9: // BUFFER_ERROR
		msg = "Buffer error"
	case 11: // CANCELLED
		msg = "Canceled"
	default:
		msg = fmt.Sprintf("Call error %d", e.Code)
	}
	s := msg + ": " + e.Detail
	if e.Timeout {
		s += " (timeout)"
	}
	return s
}

func (e *CallError) IsTimeout() bool {
	return e.Timeout
}

// Main is designed so that the complete generated main package is:
//
//      package main
//
//      import (
//              "github.com/golang/appengine/internal"
//
//              _ "myapp/package0"
//              _ "myapp/package1"
//      )
//
//      func main() {
//              internal.Main()
//      }
//
// The "myapp/packageX" packages are expected to register HTTP handlers
// in their init functions.
func Main() {
	// TODO(dsymonds): Serve to an http.Handler that intercepts and wraps HTTP requests instead.
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("http.ListenAndServe: %v", err)
	}
}

// NamespaceMods is a map from API service to a function that will mutate an RPC request to attach a namespace.
// The function should be prepared to be called on the same message more than once; it should only modify the
// RPC request the first time.
var NamespaceMods = make(map[string]func(m proto.Message, namespace string))
