// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Package aetesting provides utilities for testing App Engine packages.
// This is not for testing user applications.
package aetesting

import (
	"reflect"
	"testing"

	"code.google.com/p/goprotobuf/proto"

	"google.golang.org/appengine"
	"google.golang.org/appengine/internal"
)

// FakeSingleContext returns a context whose Call invocations will be serviced
// by f, which should be a function that has two arguments of the input and output
// protocol buffer type.
func FakeSingleContext(t *testing.T, service, method string, f interface{}) appengine.Context {
	fv := reflect.ValueOf(f)
	if fv.Kind() != reflect.Func {
		panic("not a function")
	}
	// TODO(dsymonds): check fv's signature more closely.
	return &single{
		t:       t,
		service: service,
		method:  method,
		f:       fv,
	}
}

type single struct {
	t               *testing.T
	service, method string
	f               reflect.Value
}

func (s *single) logf(level, format string, args ...interface{}) {
	s.t.Logf(level+": "+format, args...)
}

func (s *single) Debugf(format string, args ...interface{})    { s.logf("DEBUG", format, args...) }
func (s *single) Infof(format string, args ...interface{})     { s.logf("INFO", format, args...) }
func (s *single) Warningf(format string, args ...interface{})  { s.logf("WARNING", format, args...) }
func (s *single) Errorf(format string, args ...interface{})    { s.logf("ERROR", format, args...) }
func (s *single) Criticalf(format string, args ...interface{}) { s.logf("CRITICAL", format, args...) }
func (*single) FullyQualifiedAppID() string                    { return "dev~fake-app" }
func (*single) Request() interface{}                           { return nil }

func (s *single) Call(service, method string, in, out proto.Message, opts *internal.CallOptions) error {
	if service != s.service || method != s.method {
		s.t.Fatalf("Unexpected call to /%s.%s", service, method)
	}
	ins := []reflect.Value{
		reflect.ValueOf(in),
		reflect.ValueOf(out),
	}
	outs := s.f.Call(ins)
	if outs[0].IsNil() {
		return nil
	}
	return outs[0].Interface().(error)
}
