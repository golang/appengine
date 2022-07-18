// Copyright 2014 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

//go:build !appengine
// +build !appengine

package internal

import (
	"bufio"
	"bytes"
	netcontext "context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"

	basepb "google.golang.org/appengine/internal/base"
	remotepb "google.golang.org/appengine/internal/remote_api"
)

const testTicketHeader = "X-Magic-Ticket-Header"
const logserviceEnvVarKey = "LOG_TO_LOGSERVICE"

func init() {
	ticketHeader = testTicketHeader
}

type fakeAPIHandler struct {
	hang chan int // used for RunSlowly RPC

	LogFlushes int32 // atomic

	allowMissingTicket bool
}

func (f *fakeAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writeResponse := func(res *remotepb.Response) {
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
	apiReq := &remotepb.Request{}
	if err := proto.Unmarshal(hreqBody, apiReq); err != nil {
		http.Error(w, fmt.Sprintf("Bad encoded API request: %v", err), 500)
		return
	}
	if *apiReq.RequestId != "s3cr3t" && !f.allowMissingTicket {
		writeResponse(&remotepb.Response{
			RpcError: &remotepb.RpcError{
				Code:   proto.Int32(int32(remotepb.RpcError_SECURITY_VIOLATION)),
				Detail: proto.String("bad security ticket"),
			},
		})
		return
	}
	if got, want := r.Header.Get(dapperHeader), "trace-001"; got != want {
		writeResponse(&remotepb.Response{
			RpcError: &remotepb.RpcError{
				Code:   proto.Int32(int32(remotepb.RpcError_BAD_REQUEST)),
				Detail: proto.String(fmt.Sprintf("trace info = %q, want %q", got, want)),
			},
		})
		return
	}

	service, method := *apiReq.ServiceName, *apiReq.Method
	var resOut proto.Message
	if service == "actordb" && method == "LookupActor" {
		req := &basepb.StringProto{}
		res := &basepb.StringProto{}
		if err := proto.Unmarshal(apiReq.Request, req); err != nil {
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
			writeResponse(&remotepb.Response{
				RpcError: &remotepb.RpcError{
					Code:   proto.Int32(int32(remotepb.RpcError_OVER_QUOTA)),
					Detail: proto.String("you are hogging the resources!"),
				},
			})
			return
		case "RunSlowly":
			// TestAPICallRPCFailure creates f.hang, but does not strobe it
			// until Call returns with remotepb.RpcError_CANCELLED.
			// This is here to force a happens-before relationship between
			// the httptest server handler and shutdown.
			<-f.hang
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
	writeResponse(&remotepb.Response{
		Response: encOut,
	})
}

func setup() (f *fakeAPIHandler, c *context, cleanup func()) {
	f = &fakeAPIHandler{}
	srv := httptest.NewServer(f)
	u, err := url.Parse(srv.URL + apiPath)
	restoreAPIHost := restoreEnvVar("API_HOST")
	restoreAPIPort := restoreEnvVar("API_HOST")
	os.Setenv("API_HOST", u.Hostname())
	os.Setenv("API_PORT", u.Port())
	if err != nil {
		panic(fmt.Sprintf("url.Parse(%q): %v", srv.URL+apiPath, err))
	}
	return f, &context{
			req: &http.Request{
				Header: http.Header{
					ticketHeader: []string{"s3cr3t"},
					dapperHeader: []string{"trace-001"},
				},
			},
		}, func() {
			restoreAPIHost()
			restoreAPIPort()
			srv.Close()
		}
}

func restoreEnvVar(key string) (cleanup func()) {
	oldval, ok := os.LookupEnv(key)
	return func() {
		if ok {
			os.Setenv(key, oldval)
		} else {
			os.Unsetenv(key)
		}
	}
}

func TestAPICall(t *testing.T) {
	_, c, cleanup := setup()
	defer cleanup()

	req := &basepb.StringProto{
		Value: proto.String("Doctor Who"),
	}
	res := &basepb.StringProto{}
	err := Call(toContext(c), "actordb", "LookupActor", req, res)
	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}
	if got, want := *res.Value, "David Tennant"; got != want {
		t.Errorf("Response is %q, want %q", got, want)
	}
}

func TestAPICallTicketUnavailable(t *testing.T) {
	resetEnv := SetTestEnv()
	defer resetEnv()
	f, c, cleanup := setup()
	defer cleanup()
	f.allowMissingTicket = true

	c.req.Header.Set(ticketHeader, "")
	req := &basepb.StringProto{
		Value: proto.String("Doctor Who"),
	}
	res := &basepb.StringProto{}
	err := Call(toContext(c), "actordb", "LookupActor", req, res)
	if err != nil {
		t.Fatalf("API call failed: %v", err)
	}
	if got, want := *res.Value, "David Tennant"; got != want {
		t.Errorf("Response is %q, want %q", got, want)
	}
}

func TestAPICallRPCFailure(t *testing.T) {
	f, c, cleanup := setup()
	defer cleanup()

	testCases := []struct {
		method string
		code   remotepb.RpcError_ErrorCode
	}{
		{"Non200", remotepb.RpcError_UNKNOWN},
		{"ShortResponse", remotepb.RpcError_UNKNOWN},
		{"OverQuota", remotepb.RpcError_OVER_QUOTA},
		{"RunSlowly", remotepb.RpcError_CANCELLED},
	}
	f.hang = make(chan int) // only for RunSlowly
	for _, tc := range testCases {
		ctx, _ := netcontext.WithTimeout(toContext(c), 100*time.Millisecond)
		err := Call(ctx, "errors", tc.method, &basepb.VoidProto{}, &basepb.VoidProto{})
		ce, ok := err.(*CallError)
		if !ok {
			t.Errorf("%s: API call error is %T (%v), want *CallError", tc.method, err, err)
			continue
		}
		if ce.Code != int32(tc.code) {
			t.Errorf("%s: ce.Code = %d, want %d", tc.method, ce.Code, tc.code)
		}
		if tc.method == "RunSlowly" {
			f.hang <- 1 // release the HTTP handler
		}
	}
}

func TestAPICallDialFailure(t *testing.T) {
	// See what happens if the API host is unresponsive.
	// This should time out quickly, not hang forever.
	// We intentially don't set up the fakeAPIHandler for this test to cause the dail failure.
	start := time.Now()
	err := Call(netcontext.Background(), "foo", "bar", &basepb.VoidProto{}, &basepb.VoidProto{})
	const max = 1 * time.Second
	if taken := time.Since(start); taken > max {
		t.Errorf("Dial hang took too long: %v > %v", taken, max)
	}
	if err == nil {
		t.Error("Call did not fail")
	}
}

func TestDelayedLogFlushing(t *testing.T) {
	defer restoreEnvVar(logserviceEnvVarKey)()

	testCases := []struct {
		logToLogservice    string
		wantInitialFlushes int32
		wantHeader         string
		wantEndFlushes     int32
	}{
		{logToLogservice: "", wantHeader: "1", wantInitialFlushes: 1, wantEndFlushes: 2}, // default behavior
		{logToLogservice: "1", wantHeader: "1", wantInitialFlushes: 1, wantEndFlushes: 2},
		{logToLogservice: "0", wantHeader: "", wantInitialFlushes: 0, wantEndFlushes: 0},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("$%s=%q", logserviceEnvVarKey, tc.logToLogservice), func(t *testing.T) {
			f, c, cleanup := setup()
			defer cleanup()
			os.Setenv(logserviceEnvVarKey, tc.logToLogservice)

			path := "/slow_log_" + tc.logToLogservice

			http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				logC := WithContext(netcontext.Background(), r)
				Logf(logC, 1, "It's a lovely day.")
				w.WriteHeader(200)
				time.Sleep(1200 * time.Millisecond)
				w.Write(make([]byte, 100<<10)) // write 100 KB to force HTTP flush
			})

			r := &http.Request{
				Method: "GET",
				URL: &url.URL{
					Scheme: "http",
					Path:   path,
				},
				Header: c.req.Header,
				Body:   ioutil.NopCloser(bytes.NewReader(nil)),
			}
			w := httptest.NewRecorder()

			handled := make(chan struct{})
			go func() {
				defer close(handled)
				Middleware(http.DefaultServeMux).ServeHTTP(w, r)
			}()
			// Check that the log flush eventually comes in.
			time.Sleep(1200 * time.Millisecond)
			if got := atomic.LoadInt32(&f.LogFlushes); got != tc.wantInitialFlushes {
				t.Errorf("After 1.2s: f.LogFlushes = %d, want %d", got, tc.wantInitialFlushes)
			}

			<-handled
			const hdr = "X-AppEngine-Log-Flush-Count"
			if got := w.HeaderMap.Get(hdr); got != tc.wantHeader {
				t.Errorf("%s header = %q, want %q", hdr, got, tc.wantHeader)
			}
			if got := atomic.LoadInt32(&f.LogFlushes); got != tc.wantEndFlushes {
				t.Errorf("After HTTP response: f.LogFlushes = %d, want %d", got, tc.wantEndFlushes)
			}
		})
	}
}

func TestLogFlushing(t *testing.T) {
	defer restoreEnvVar(logserviceEnvVarKey)()

	testCases := []struct {
		logToLogservice string
		wantHeader      string
		wantFlushes     int32
	}{
		{logToLogservice: "", wantHeader: "1", wantFlushes: 1}, // default behavior
		{logToLogservice: "1", wantHeader: "1", wantFlushes: 1},
		{logToLogservice: "0", wantHeader: "", wantFlushes: 0},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("$%s=%q", logserviceEnvVarKey, tc.logToLogservice), func(t *testing.T) {
			f, c, cleanup := setup()
			defer cleanup()
			os.Setenv(logserviceEnvVarKey, tc.logToLogservice)

			path := "/quick_log_" + tc.logToLogservice
			http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				logC := WithContext(netcontext.Background(), r)
				Logf(logC, 1, "It's a lovely day.")
				w.WriteHeader(200)
				w.Write(make([]byte, 100<<10)) // write 100 KB to force HTTP flush
			})

			r := &http.Request{
				Method: "GET",
				URL: &url.URL{
					Scheme: "http",
					Path:   path,
				},
				Header: c.req.Header,
				Body:   ioutil.NopCloser(bytes.NewReader(nil)),
			}
			w := httptest.NewRecorder()

			Middleware(http.DefaultServeMux).ServeHTTP(w, r)
			const hdr = "X-AppEngine-Log-Flush-Count"
			if got := w.HeaderMap.Get(hdr); got != tc.wantHeader {
				t.Errorf("%s header = %q, want %q", hdr, got, tc.wantHeader)
			}
			if got := atomic.LoadInt32(&f.LogFlushes); got != tc.wantFlushes {
				t.Errorf("After HTTP response: f.LogFlushes = %d, want %d", got, tc.wantFlushes)
			}
		})
	}
}

func TestRemoteAddr(t *testing.T) {
	var addr string
	http.HandleFunc("/remote_addr", func(w http.ResponseWriter, r *http.Request) {
		addr = r.RemoteAddr
	})

	testCases := []struct {
		headers http.Header
		addr    string
	}{
		{http.Header{"X-Appengine-User-Ip": []string{"10.5.2.1"}}, "10.5.2.1:80"},
		{http.Header{"X-Appengine-Remote-Addr": []string{"1.2.3.4"}}, "1.2.3.4:80"},
		{http.Header{"X-Appengine-Remote-Addr": []string{"1.2.3.4:8080"}}, "1.2.3.4:8080"},
		{
			http.Header{"X-Appengine-Remote-Addr": []string{"2401:fa00:9:1:7646:a0ff:fe90:ca66"}},
			"[2401:fa00:9:1:7646:a0ff:fe90:ca66]:80",
		},
		{
			http.Header{"X-Appengine-Remote-Addr": []string{"[::1]:http"}},
			"[::1]:http",
		},
		{http.Header{}, "127.0.0.1:80"},
	}

	for _, tc := range testCases {
		r := &http.Request{
			Method: "GET",
			URL:    &url.URL{Scheme: "http", Path: "/remote_addr"},
			Header: tc.headers,
			Body:   ioutil.NopCloser(bytes.NewReader(nil)),
		}
		Middleware(http.DefaultServeMux).ServeHTTP(httptest.NewRecorder(), r)
		if addr != tc.addr {
			t.Errorf("Header %v, got %q, want %q", tc.headers, addr, tc.addr)
		}
	}
}

func TestPanickingHandler(t *testing.T) {
	http.HandleFunc("/panic", func(http.ResponseWriter, *http.Request) {
		panic("whoops!")
	})
	r := &http.Request{
		Method: "GET",
		URL:    &url.URL{Scheme: "http", Path: "/panic"},
		Body:   ioutil.NopCloser(bytes.NewReader(nil)),
	}
	rec := httptest.NewRecorder()
	Middleware(http.DefaultServeMux).ServeHTTP(rec, r)
	if rec.Code != 500 {
		t.Errorf("Panicking handler returned HTTP %d, want HTTP %d", rec.Code, 500)
	}
}

var raceDetector = false

func TestAPICallAllocations(t *testing.T) {
	if raceDetector {
		t.Skip("not running under race detector")
	}

	// Run the test API server in a subprocess so we aren't counting its allocations.
	cleanup := launchHelperProcess(t)

	defer cleanup()
	c := &context{
		req: &http.Request{
			Header: http.Header{
				ticketHeader: []string{"s3cr3t"},
				dapperHeader: []string{"trace-001"},
			},
		},
	}

	req := &basepb.StringProto{
		Value: proto.String("Doctor Who"),
	}
	res := &basepb.StringProto{}
	var apiErr error
	avg := testing.AllocsPerRun(100, func() {
		ctx, _ := netcontext.WithTimeout(toContext(c), 100*time.Millisecond)
		if err := Call(ctx, "actordb", "LookupActor", req, res); err != nil && apiErr == nil {
			apiErr = err // get the first error only
		}
	})
	if apiErr != nil {
		t.Errorf("API call failed: %v", apiErr)
	}

	// Lots of room for improvement...
	var min, max float64 = 60, 86
	if strings.HasPrefix(runtime.Version(), "go1.11.") || strings.HasPrefix(runtime.Version(), "go1.12.") {
		// add a bit more overhead for versions before go1.13
		// see https://go.dev/doc/go1.13#compilers
		max = 90
	}
	if avg < min || max < avg {
		t.Errorf("Allocations per API call = %g, want in [%g,%g]", avg, min, max)
	}
}

func launchHelperProcess(t *testing.T) (cleanup func()) {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Starting helper process: %v", err)
	}

	scan := bufio.NewScanner(stdout)
	var u *url.URL
	for scan.Scan() {
		line := scan.Text()
		if hp := strings.TrimPrefix(line, helperProcessMagic); hp != line {
			var err error
			u, err = url.Parse(hp)
			if err != nil {
				t.Fatalf("Failed to parse %q: %v", hp, err)
			}
			break
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("Scanning helper process stdout: %v", err)
	}
	if u == nil {
		t.Fatal("Helper process never reported")
	}

	restoreAPIHost := restoreEnvVar("API_HOST")
	restoreAPIPort := restoreEnvVar("API_HOST")
	os.Setenv("API_HOST", u.Hostname())
	os.Setenv("API_PORT", u.Port())
	return func() {
		restoreAPIHost()
		restoreAPIPort()
		stdin.Close()
		if err := cmd.Wait(); err != nil {
			t.Errorf("Helper process did not exit cleanly: %v", err)
		}
	}
}

const helperProcessMagic = "A lovely helper process is listening at "

// This isn't a real test. It's used as a helper process.
func TestHelperProcess(*testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	f := &fakeAPIHandler{}
	srv := httptest.NewServer(f)
	defer srv.Close()
	fmt.Println(helperProcessMagic + srv.URL + apiPath)

	// Wait for stdin to be closed.
	io.Copy(ioutil.Discard, os.Stdin)
}
