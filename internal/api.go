// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package internal

import (
	"errors"
	"log"
	"net/http"

	"code.google.com/p/goprotobuf/proto"
)

func call(service, method string, data []byte, requestID string) ([]byte, error) {
	return nil, errors.New("TODO: API calls")
}

// context represents the context of an in-flight HTTP request.
// It implements the appengine.Context interface.
type context struct {
	req *http.Request
}

func NewContext(req *http.Request) *context {
	return &context{req: req}
}

func (c *context) Call(service, method string, in, out proto.Message, opts *CallOptions) error {
	/* TODO
	if service == "__go__" {
		if method == "GetNamespace" {
			out.(*basepb.StringProto).Value = proto.String(c.req.Header.Get("X-AppEngine-Current-Namespace"))
			return nil
		}
		if method == "GetDefaultNamespace" {
			out.(*basepb.StringProto).Value = proto.String(c.req.Header.Get("X-AppEngine-Default-Namespace"))
			return nil
		}
	}
	*/
	data, err := proto.Marshal(in)
	if err != nil {
		return err
	}

	// TODO(dsymonds): this isn't right
	requestID := c.req.Header.Get("X-Appengine-Internal-Request-Id")
	res, err := call(service, method, data, requestID)
	if err != nil {
		return err
	}
	return proto.Unmarshal(res, out)
}

func (c *context) Request() interface{} {
	return c.req
}

func (c *context) logf(level, format string, args ...interface{}) {
	// TODO(dsymonds): This isn't complete.
	log.Printf(level+": "+format, args...)
}

func (c *context) Debugf(format string, args ...interface{})    { c.logf("DEBUG", format, args...) }
func (c *context) Infof(format string, args ...interface{})     { c.logf("INFO", format, args...) }
func (c *context) Warningf(format string, args ...interface{})  { c.logf("WARNING", format, args...) }
func (c *context) Errorf(format string, args ...interface{})    { c.logf("ERROR", format, args...) }
func (c *context) Criticalf(format string, args ...interface{}) { c.logf("CRITICAL", format, args...) }

// FullyQualifiedAppID returns the fully-qualified application ID.
// This may contain a partition prefix (e.g. "s~" for High Replication apps),
// or a domain prefix (e.g. "example.com:").
func (c *context) FullyQualifiedAppID() string {
	return "s~todo" // TODO
}
