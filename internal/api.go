// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package internal

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"code.google.com/p/goprotobuf/proto"
)

var (
	portFlag = flag.Int("port", 0, "HTTP service port")
)

// serveHTTP serves App Engine HTTP requests.
func serveHTTP() {
	err := http.ListenAndServe(fmt.Sprintf(":%d", *portFlag), http.HandlerFunc(handleFilteredHTTP))
	if err != nil {
		log.Fatal("appengine: ", err)
	}
}

func handleFilteredHTTP(w http.ResponseWriter, r *http.Request) {
	// Create a private copy of the Request that includes headers that are
	// private to the runtime and strip those headers from the request that the
	// user application sees.
	creq := *r
	r.Header = make(http.Header)
	for name, values := range creq.Header {
		if !strings.HasPrefix(name, "X-Appengine-Internal-") {
			r.Header[name] = values
		}
	}
	ctxsMu.Lock()
	ctxs[r] = &context{req: &creq}
	ctxsMu.Unlock()

	http.DefaultServeMux.ServeHTTP(w, r)

	ctxsMu.Lock()
	delete(ctxs, r)
	ctxsMu.Unlock()
}

var (
	ctxsMu sync.Mutex
	ctxs   = make(map[*http.Request]*context)
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
	ctxsMu.Lock()
	defer ctxsMu.Unlock()
	c := ctxs[req]

	if c == nil {
		// Someone passed in an http.Request that is not in-flight.
		// We panic here rather than panicking at a later point
		// so that backtraces will be more sensible.
		log.Panic("appengine: NewContext passed an unknown http.Request")
	}
	return c
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
