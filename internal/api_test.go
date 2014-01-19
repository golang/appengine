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
		// TODO: APIResponse with SECURITY_VIOLATION instead.
		http.Error(w, fmt.Sprintf("Bad security ticket"), 500)
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

	encOut, err := proto.Marshal(resOut)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed encoding response: %v", err), 500)
		return
	}
	apiRes := &runtimepb.APIResponse{
		Error: proto.Int32(int32(runtimepb.APIResponse_OK)),
		Pb:    encOut,
	}
	hresBody, err := proto.Marshal(apiRes)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed encoding API response: %v", err), 500)
		return
	}
	w.Write(hresBody)
}

func TestAPICall(t *testing.T) {
	defer func(orig string) { apiHost = orig }(apiHost)

	srv := httptest.NewServer(http.HandlerFunc(fakeAPIHandler))
	defer srv.Close()
	apiHost = strings.TrimPrefix(srv.URL, "http://")

	c := NewContext(&http.Request{
		Header: http.Header{
			ticketHeader: []string{"s3cr3t"},
		},
	})
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
