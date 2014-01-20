// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"code.google.com/p/goprotobuf/proto"
	runtimepb "github.com/golang/appengine/internal/runtime"
)

const (
	apiPath = "/rpc_http"
)

var (
	apiHost = "appengine.googleapis.com:10001" // var for testing

	// Incoming headers.
	ticketHeader = os.Getenv("HTTP_X_APPENGINE_API_TICKET")

	// Outgoing headers.
	apiEndpointHeader      = http.CanonicalHeaderKey("X-Google-RPC-Service-Endpoint")
	apiEndpointHeaderValue = []string{"app-engine-apis"}
	apiMethodHeader        = http.CanonicalHeaderKey("X-Google-RPC-Service-Method")
	apiMethodHeaderValue   = []string{"/APIHost.Call"}
	apiContentType         = http.CanonicalHeaderKey("Content-Type")
	apiContentTypeValue    = []string{"application/octet-stream"}
)

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

	ticket := c.req.Header.Get(ticketHeader)
	req := &runtimepb.APIRequest{
		ApiPackage:     &service,
		Call:           &method,
		Pb:             data,
		SecurityTicket: &ticket,
	}
	hreqBody, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	// TODO(dsymonds): deadline handling, trace info

	hreq := &http.Request{
		Method: "POST",
		URL: &url.URL{
			Scheme: "http",
			Host:   apiHost,
			Path:   apiPath,
		},
		Header: http.Header{
			apiEndpointHeader: apiEndpointHeaderValue,
			apiMethodHeader:   apiMethodHeaderValue,
			apiContentType:    apiContentTypeValue,
		},
		Body:          ioutil.NopCloser(bytes.NewReader(hreqBody)),
		ContentLength: int64(len(hreqBody)),
		Host:          apiHost,
	}

	hresp, err := http.DefaultClient.Do(hreq)
	if err != nil {
		// TODO(dsymonds): Check for timeout, return CallError with Timeout=true.
		return &CallError{
			Detail: fmt.Sprintf("service bridge HTTP failed: %v", err),
			Code:   int32(runtimepb.APIResponse_RPC_ERROR),
		}
	}
	defer hresp.Body.Close()
	if hresp.StatusCode != 200 {
		return &CallError{
			Detail: fmt.Sprintf("service bridge returned HTTP %d", hresp.StatusCode),
			Code:   int32(runtimepb.APIResponse_RPC_ERROR),
		}
	}
	hrespBody, err := ioutil.ReadAll(hresp.Body)
	if err != nil {
		return &CallError{
			Detail: fmt.Sprintf("service bridge response bad: %v", err),
			Code:   int32(runtimepb.APIResponse_RPC_ERROR),
		}
	}

	res := &runtimepb.APIResponse{}
	if err := proto.Unmarshal(hrespBody, res); err != nil {
		return err
	}
	if *res.Error != int32(runtimepb.APIResponse_OK) {
		if *res.Error == int32(runtimepb.APIResponse_RPC_ERROR) {
			switch res.GetRpcError() {
			case runtimepb.APIResponse_DEADLINE_EXCEEDED:
				// TODO(dsymonds): Add a DEADLINE_EXCEEDED error code?
				return &CallError{
					Detail:  "Deadline exceeded",
					Code:    int32(runtimepb.APIResponse_CANCELLED),
					Timeout: true,
				}
			case runtimepb.APIResponse_APPLICATION_ERROR:
				return &APIError{
					Service: *req.ApiPackage,
					Detail:  res.GetErrorMessage(),
					Code:    res.GetRpcApplicationError(),
				}

			}
		}
		return &CallError{
			Detail: res.GetErrorMessage(),
			Code:   *res.Error,
		}
	}
	return proto.Unmarshal(res.Pb, out)
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
