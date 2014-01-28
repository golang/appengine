// Copyright 2014 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"code.google.com/p/goprotobuf/proto"

	basepb "github.com/golang/appengine/internal/base"
	runtimepb "github.com/golang/appengine/internal/runtime"
)

const testTicketHeader = "X-Magic-Ticket-Header"

func init() {
	ticketHeader = testTicketHeader
}

func fakeAPIHandler(w http.ResponseWriter, r *http.Request) {
	writeAPIResponse := func(res *runtimepb.APIResponse) {
		hresBody, err := proto.Marshal(res)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed encoding API response: %v", err), 500)
			return
		}
		w.Write(hresBody)
	}

	if r.URL.Path != "/rpc_http" {
		http.NotFound(w, r)
		return
	}
	hreqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bad body: %v", err), 500)
		return
	}
	apiReq := &runtimepb.APIRequest{}
	if err := proto.Unmarshal(hreqBody, apiReq); err != nil {
		http.Error(w, fmt.Sprintf("Bad encoded API request: %v", err), 500)
		return
	}
	if *apiReq.SecurityTicket != "s3cr3t" {
		writeAPIResponse(&runtimepb.APIResponse{
			Error:        proto.Int32(int32(runtimepb.APIResponse_SECURITY_VIOLATION)),
			ErrorMessage: proto.String("bad security ticket"),
		})
		return
	}

	service, method := *apiReq.ApiPackage, *apiReq.Call
	var resOut proto.Message
	if service == "actordb" && method == "LookupActor" {
		req := &basepb.StringProto{}
		res := &basepb.StringProto{}
		if err := proto.Unmarshal(apiReq.Pb, req); err != nil {
			http.Error(w, fmt.Sprintf("Bad encoded request: %v", err), 500)
			return
		}
		if *req.Value == "Doctor Who" {
			res.Value = proto.String("David Tennant")
		}
		resOut = res
	}
	if service == "errors" {
		switch method {
		case "Non200":
			http.Error(w, "I'm a little teapot.", 418)
			return
		case "ShortResponse":
			w.Header().Set("Content-Length", "100")
			w.Write([]byte("way too short"))
			return
		case "OverQuota":
			writeAPIResponse(&runtimepb.APIResponse{
				Error:        proto.Int32(int32(runtimepb.APIResponse_OVER_QUOTA)),
				ErrorMessage: proto.String("you are hogging the resources!"),
			})
			return
		}
	}

	encOut, err := proto.Marshal(resOut)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed encoding response: %v", err), 500)
		return
	}
	writeAPIResponse(&runtimepb.APIResponse{
		Error: proto.Int32(int32(runtimepb.APIResponse_OK)),
		Pb:    encOut,
	})
}

func setup() (c *context, cleanup func()) {
	origAPIHost := apiHost
	srv := httptest.NewServer(http.HandlerFunc(fakeAPIHandler))
	apiHost = strings.TrimPrefix(srv.URL, "http://")
	return &context{
			req: &http.Request{
				Header: http.Header{
					ticketHeader: []string{"s3cr3t"},
				},
			},
		}, func() {
			srv.Close()
			apiHost = origAPIHost
		}
}

func TestAPICall(t *testing.T) {
	c, cleanup := setup()
	defer cleanup()

	req := &basepb.StringProto{
		Value: proto.String("Doctor Who"),
	}
	res := &basepb.StringProto{}
	err := c.Call("actordb", "LookupActor", req, res, nil)
	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}
	if got, want := *res.Value, "David Tennant"; got != want {
		t.Errorf("Response is %q, want %q", got, want)
	}
}

func TestAPICallRPCFailure(t *testing.T) {
	c, cleanup := setup()
	defer cleanup()

	testCases := []struct {
		method string
		code   runtimepb.APIResponse_ERROR
	}{
		{"Non200", runtimepb.APIResponse_RPC_ERROR},
		{"ShortResponse", runtimepb.APIResponse_RPC_ERROR},
		{"OverQuota", runtimepb.APIResponse_OVER_QUOTA},
	}
	for _, tc := range testCases {
		err := c.Call("errors", tc.method, &basepb.VoidProto{}, &basepb.VoidProto{}, nil)
		ce, ok := err.(*CallError)
		if !ok {
			t.Errorf("%s: API call error is %T (%v), want *CallError", tc.method, err, err)
			continue
		}
		if ce.Code != int32(tc.code) {
			t.Errorf("%s: ce.Code = %d, want %d", tc.method, ce.Code, tc.code)
		}
	}
}
