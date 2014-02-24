// Copyright 2014 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"code.google.com/p/goprotobuf/proto"

	basepb "google.golang.org/appengine/internal/base"
	runtimepb "google.golang.org/appengine/internal/runtime"
)

const testTicketHeader = "X-Magic-Ticket-Header"

func init() {
	ticketHeader = testTicketHeader
}

type fakeAPIHandler struct {
	die chan int // closed when the test server is going down

	LogFlushes int32 // atomic
}

func (f *fakeAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	if got, want := r.Header.Get(dapperHeader), "trace-001"; got != want {
		writeAPIResponse(&runtimepb.APIResponse{
			Error:        proto.Int32(int32(runtimepb.APIResponse_BAD_REQUEST)),
			ErrorMessage: proto.String(fmt.Sprintf("trace info = %q, want %q", got, want)),
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
		case "RunSlowly":
			// Avoid blocking test shutdown by aborting early when the test is over.
			select {
			case <-time.After(5 * time.Second):
			case <-f.die:
				return
			}
			resOut = &basepb.VoidProto{}
		}
	}
	if service == "logservice" && method == "Flush" {
		// Pretend log flushing is slow.
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&f.LogFlushes, 1)
		resOut = &basepb.VoidProto{}
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

func setup() (f *fakeAPIHandler, c *context, cleanup func()) {
	origAPIHost := apiHost
	f = &fakeAPIHandler{
		die: make(chan int),
	}
	srv := httptest.NewServer(f)
	apiHost = strings.TrimPrefix(srv.URL, "http://")
	return f, &context{
			req: &http.Request{
				Header: http.Header{
					ticketHeader: []string{"s3cr3t"},
					dapperHeader: []string{"trace-001"},
				},
			},
		}, func() {
			close(f.die)
			srv.Close()
			apiHost = origAPIHost
		}
}

func TestAPICall(t *testing.T) {
	_, c, cleanup := setup()
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
	_, c, cleanup := setup()
	defer cleanup()

	testCases := []struct {
		method string
		code   runtimepb.APIResponse_ERROR
	}{
		{"Non200", runtimepb.APIResponse_RPC_ERROR},
		{"ShortResponse", runtimepb.APIResponse_RPC_ERROR},
		{"OverQuota", runtimepb.APIResponse_OVER_QUOTA},
		{"RunSlowly", runtimepb.APIResponse_CANCELLED},
	}
	for _, tc := range testCases {
		opts := &CallOptions{
			Timeout: 100 * time.Millisecond,
		}
		err := c.Call("errors", tc.method, &basepb.VoidProto{}, &basepb.VoidProto{}, opts)
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

func TestAPICallDialFailure(t *testing.T) {
	// See what happens if the API host is unresponsive.
	// This should time out quickly, not hang forever.
	_, c, cleanup := setup()
	defer cleanup()
	apiHost = "appengine.googleapis.com:10001"

	start := time.Now()
	err := c.Call("foo", "bar", &basepb.VoidProto{}, &basepb.VoidProto{}, nil)
	const max = 1 * time.Second
	if taken := time.Since(start); taken > max {
		t.Errorf("Dial hang took too long: %v > %v", taken, max)
	}
	if err == nil {
		t.Error("Call did not fail")
	}
}

func TestDelayedLogFlushing(t *testing.T) {
	f, c, cleanup := setup()
	defer cleanup()

	http.HandleFunc("/quick_log", func(w http.ResponseWriter, r *http.Request) {
		c := NewContext(r)
		c.Infof("It's a lovely day.")
		w.WriteHeader(200)
		w.Write(make([]byte, 100<<10)) // write 100 KB to force HTTP flush
	})

	r := &http.Request{
		Method: "GET",
		URL: &url.URL{
			Scheme: "http",
			Path:   "/quick_log",
		},
		Header: c.req.Header,
		Body:   ioutil.NopCloser(bytes.NewReader(nil)),
	}
	w := httptest.NewRecorder()

	// Check that log flushing does not hold up the HTTP response.
	start := time.Now()
	handleHTTP(w, r)
	if d := time.Since(start); d > 10*time.Millisecond {
		t.Errorf("handleHTTP took %v, want under 10ms", d)
	}
	const hdr = "X-AppEngine-Log-Flush-Count"
	if h := w.HeaderMap.Get(hdr); h != "1" {
		t.Errorf("%s header = %q, want %q", hdr, h, "1")
	}
	if f := atomic.LoadInt32(&f.LogFlushes); f != 0 {
		t.Errorf("After HTTP response: f.LogFlushes = %d, want 0", f)
	}

	// Check that the log flush eventually comes in.
	time.Sleep(100 * time.Millisecond)
	if f := atomic.LoadInt32(&f.LogFlushes); f != 1 {
		t.Errorf("After 100ms: f.LogFlushes = %d, want 1", f)
	}
}
