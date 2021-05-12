// Copyright 2014 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package internal

import (
	"bufio"
	"bytes"
	netcontext "context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"

	basepb "google.golang.org/appengine/internal/base"
	remotepb "google.golang.org/appengine/internal/remote_api"
)

const testTicketHeader = "X-Magic-Ticket-Header"

func init() {
	ticketHeader = testTicketHeader
}

type fakeAPIHandler struct {
	hang chan int // used for RunSlowly RPC

	LogFlushes int32 // atomic
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
	if *apiReq.RequestId != "s3cr3t" && *apiReq.RequestId != DefaultTicket() {
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
		apiURL: u,
	}, srv.Close
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
	_, c, cleanup := setup()
	defer cleanup()

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
	_, c, cleanup := setup()
	defer cleanup()
	// Reset the URL to the production address so that dialing fails.
	c.apiURL = apiURL()

	start := time.Now()
	err := Call(toContext(c), "foo", "bar", &basepb.VoidProto{}, &basepb.VoidProto{})
	const max = 1 * time.Second
	if taken := time.Since(start); taken > max {
		t.Errorf("Dial hang took too long: %v > %v", taken, max)
	}
	if err == nil {
		t.Error("Call did not fail")
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
		handleHTTP(httptest.NewRecorder(), r)
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
	handleHTTP(rec, r)
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
	u, cleanup := launchHelperProcess(t)
	defer cleanup()
	c := &context{
		req: &http.Request{
			Header: http.Header{
				ticketHeader: []string{"s3cr3t"},
				dapperHeader: []string{"trace-001"},
			},
		},
		apiURL: u,
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
	const min, max float64 = 60, 86
	if avg < min || max < avg {
		t.Errorf("Allocations per API call = %g, want in [%g,%g]", avg, min, max)
	}
}

func launchHelperProcess(t *testing.T) (apiURL *url.URL, cleanup func()) {
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

	return u, func() {
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

func TestBackgroundContext(t *testing.T) {
	resetEnv := SetTestEnv()
	defer resetEnv()

	ctx, key := fromContext(BackgroundContext()), "X-Magic-Ticket-Header"
	if g, w := ctx.req.Header.Get(key), "my-app-id/default.20150612t184001.0"; g != w {
		t.Errorf("%v = %q, want %q", key, g, w)
	}

	// Check that using the background context doesn't panic.
	req := &basepb.StringProto{
		Value: proto.String("Doctor Who"),
	}
	res := &basepb.StringProto{}
	Call(BackgroundContext(), "actordb", "LookupActor", req, res) // expected to fail
}

func TestLogf(t *testing.T) {

	testCases := []struct {
		name     string
		deployed bool
		level    int64
		format   string
		args     []interface{}
		want     string
		wantJSON bool
	}{
		{
			name:   "local-debug",
			level:  0,
			format: "my %s %d",
			args:   []interface{}{"abc", 1},
			want:   "DEBUG: my abc 1\n",
		},
		{
			name:   "local-info",
			level:  1,
			format: "my %s %d",
			args:   []interface{}{"abc", 1},
			want:   "INFO: my abc 1\n",
		},
		{
			name:   "local-warning",
			level:  2,
			format: "my %s %d",
			args:   []interface{}{"abc", 1},
			want:   "WARNING: my abc 1\n",
		},
		{
			name:   "local-error",
			level:  3,
			format: "my %s %d",
			args:   []interface{}{"abc", 1},
			want:   "ERROR: my abc 1\n",
		},
		{
			name:   "local-critical",
			level:  4,
			format: "my %s %d",
			args:   []interface{}{"abc", 1},
			want:   "CRITICAL: my abc 1\n",
		},
		{
			name:   "local-multiline",
			level:  0,
			format: "my \n multiline\n\n",
			want:   "DEBUG: my \n multiline\n",
		},
		{
			name:     "deployed-plain-debug",
			deployed: true,
			level:    0,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message": "my abc 1", "severity": "DEBUG"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-info",
			deployed: true,
			level:    1,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message": "my abc 1", "severity": "INFO"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-warning",
			deployed: true,
			level:    2,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message": "my abc 1", "severity": "WARNING"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-error",
			deployed: true,
			level:    3,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message": "my abc 1", "severity": "ERROR"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-critical",
			deployed: true,
			level:    4,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message": "my abc 1", "severity": "CRITICAL"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-multiline",
			deployed: true,
			level:    0,
			format:   "my \n multiline\n\n",
			want:     "{\"message\": \"my \\n multiline\\n\\n\", \"severity\": \"DEBUG\"}\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-megaquote",
			deployed: true,
			level:    0,
			format:   `my "megaquote" %q`,
			args:     []interface{}{`internal "quote"`},
			want:     "{\"message\": \"my \\\"megaquote\\\" \\\"internal \\\\\\\"quote\\\\\\\"\\\"\", \"severity\": \"DEBUG\"}\n",
			wantJSON: true,
		},
		{
			name:     "deployed-structured-debug",
			deployed: true,
			level:    0,
			format:   `{"some": "message %s %d"}`,
			args:     []interface{}{"abc", 1},
			want:     `{"some": "message abc 1"}` + "\n",
		},
		{
			name:     "deployed-structured-info",
			deployed: true,
			level:    1,
			format:   `{"some": "message %s %d"}`,
			args:     []interface{}{"abc", 1},
			want:     `{"some": "message abc 1"}` + "\n",
		},
		{
			name:     "deployed-structured-warning",
			deployed: true,
			level:    2,
			format:   `{"some": "message %s %d"}`,
			args:     []interface{}{"abc", 1},
			want:     `{"some": "message abc 1"}` + "\n",
		},
		{
			name:     "deployed-structured-error",
			deployed: true,
			level:    3,
			format:   `{"some": "message %s %d"}`,
			args:     []interface{}{"abc", 1},
			want:     `{"some": "message abc 1"}` + "\n",
		},
		{
			name:     "deployed-structured-critical",
			deployed: true,
			level:    4,
			format:   `{"some": "message %s %d"}`,
			args:     []interface{}{"abc", 1},
			want:     `{"some": "message abc 1"}` + "\n",
		},
		{
			// The leading "{" assumes this is already a structured log, so no alteration is performed.
			name:     "deployed-structured-multiline",
			deployed: true,
			level:    4,
			// This is not even valid JSON; we don't attempt to validate and only use the first character.
			format: "{\"some\": \"message\n%s %d\"",
			args:   []interface{}{"abc", 1},
			want:   "{\"some\": \"message\nabc 1\"\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := ""
			if tc.deployed {
				env = "standard"
			}
			defer setEnvVar(t, "GAE_ENV", env)()
			var got string
			defer overrideLogPrint(t, &got)()
			ctx := fromContext(BackgroundContext())

			logf(ctx, tc.level, tc.format, tc.args...)

			if got != tc.want {
				t.Errorf("incorrect log got=%q want=%q", got, tc.want)
			}

			if tc.wantJSON {
				var e struct {
					Message  string `json:"message"`
					Severity string `json:"severity"`
				}
				if err := json.Unmarshal([]byte(got), &e); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if gotMsg, wantMsg := e.Message, fmt.Sprintf(tc.format, tc.args...); gotMsg != wantMsg {
					t.Errorf("JSON-encoded message incorrect got=%q want=%q", gotMsg, wantMsg)
				}
				if gotSev, wantSev := e.Severity, logLevelName[tc.level]; gotSev != wantSev {
					t.Errorf("JSON-encoded severity incorrect got=%q want=%q", gotSev, wantSev)
				}
			}
		})
	}
}

func setEnvVar(t *testing.T, key, value string) func() {
	t.Helper()
	old, present := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatal(err)
	}
	return func() {
		if present {
			if err := os.Setenv(key, old); err != nil {
				t.Fatal(err)
			}
			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func overrideLogPrint(t *testing.T, output *string) func() {
	t.Helper()
	old := logPrint
	logPrint = func(args ...interface{}) {
		if len(args) != 1 {
			t.Fatal("expected exactly 1 arg")
		}
		s, ok := args[0].(string)
		if !ok {
			t.Fatalf("expected string, got %T", args[0])
		}
		*output = s
	}
	return func() { logPrint = old }
}
