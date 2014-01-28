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
	"strings"
	"sync"
	"time"

	"code.google.com/p/goprotobuf/proto"
	basepb "github.com/golang/appengine/internal/base"
	logpb "github.com/golang/appengine/internal/log"
	runtimepb "github.com/golang/appengine/internal/runtime"
)

const (
	apiPath = "/rpc_http"
)

var (
	apiHost = "appengine.googleapis.com:10001" // var for testing

	// Incoming headers.
	ticketHeader = http.CanonicalHeaderKey("X-AppEngine-API-Ticket")

	// Outgoing headers.
	apiEndpointHeader      = http.CanonicalHeaderKey("X-Google-RPC-Service-Endpoint")
	apiEndpointHeaderValue = []string{"app-engine-apis"}
	apiMethodHeader        = http.CanonicalHeaderKey("X-Google-RPC-Service-Method")
	apiMethodHeaderValue   = []string{"/APIHost.Call"}
	apiContentType         = http.CanonicalHeaderKey("Content-Type")
	apiContentTypeValue    = []string{"application/octet-stream"}
)

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	c := &context{req: r}
	stopFlushing := make(chan int)

	ctxs.Lock()
	ctxs.m[r] = c
	ctxs.Unlock()
	defer func() {
		stopFlushing <- 1 // any logging beyond this point will be dropped
		c.flushLog(false) // flush any pending logs

		ctxs.Lock()
		delete(ctxs.m, r)
		ctxs.Unlock()
	}()

	// Start goroutine responsible for flushing app logs.
	// This is done after adding c to ctx.m (and stopped before removing it)
	// because flushing logs requires making an API call.
	go c.logFlusher(stopFlushing)

	http.DefaultServeMux.ServeHTTP(w, r)
}

var ctxs = struct {
	sync.Mutex
	m map[*http.Request]*context
}{
	m: make(map[*http.Request]*context),
}

// context represents the context of an in-flight HTTP request.
// It implements the appengine.Context interface.
type context struct {
	req *http.Request

	pendingLogs struct {
		sync.Mutex
		lines []*logpb.UserAppLogLine
	}
}

func NewContext(req *http.Request) *context {
	ctxs.Lock()
	c := ctxs.m[req]
	ctxs.Unlock()

	if c == nil {
		// Someone passed in an http.Request that is not in-flight.
		// We panic here rather than panicking at a later point
		// so that stack traces will be more sensible.
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

func (c *context) addLogLine(ll *logpb.UserAppLogLine) {
	// Truncate long log lines.
	// TODO(dsymonds): Check if this is still necessary.
	const lim = 8 << 10
	if len(*ll.Message) > lim {
		suffix := fmt.Sprintf("...(length %d)", len(*ll.Message))
		ll.Message = proto.String((*ll.Message)[:lim-len(suffix)] + suffix)
	}

	c.pendingLogs.Lock()
	c.pendingLogs.lines = append(c.pendingLogs.lines, ll)
	c.pendingLogs.Unlock()
}

var logLevelName = map[int64]string{
	0: "DEBUG",
	1: "INFO",
	2: "WARNING",
	3: "ERROR",
	4: "CRITICAL",
}

func (c *context) logf(level int64, format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	s = strings.TrimRight(s, "\n") // Remove any trailing newline characters.
	c.addLogLine(&logpb.UserAppLogLine{
		TimestampUsec: proto.Int64(time.Now().UnixNano() / 1e3),
		Level:         &level,
		Message:       &s,
	})
	log.Print(logLevelName[level] + ": " + s)
}

func (c *context) Debugf(format string, args ...interface{})    { c.logf(0, format, args...) }
func (c *context) Infof(format string, args ...interface{})     { c.logf(1, format, args...) }
func (c *context) Warningf(format string, args ...interface{})  { c.logf(2, format, args...) }
func (c *context) Errorf(format string, args ...interface{})    { c.logf(3, format, args...) }
func (c *context) Criticalf(format string, args ...interface{}) { c.logf(4, format, args...) }

// FullyQualifiedAppID returns the fully-qualified application ID.
// This may contain a partition prefix (e.g. "s~" for High Replication apps),
// or a domain prefix (e.g. "example.com:").
func (c *context) FullyQualifiedAppID() string {
	// TODO(dsymonds): Memoize this.

	// gae_project has everything except the partition prefix.
	appID := string(mustGetMetadata("instance/attributes/gae_project"))
	if part := string(mustGetMetadata("instance/attributes/gae_partition")); part != "" {
		appID = part + "~" + appID
	}

	return appID
}

// flushLog attempts to flush any pending logs to the appserver.
// It should not be called concurrently.
func (c *context) flushLog(force bool) (flushed bool) {
	c.pendingLogs.Lock()
	// Grab up to 30 MB. We can get away with up to 32 MB, but let's be cautious.
	n, rem := 0, 30<<20
	for ; n < len(c.pendingLogs.lines); n++ {
		ll := c.pendingLogs.lines[n]
		// Each log line will require about 3 bytes of overhead.
		nb := proto.Size(ll) + 3
		if nb > rem {
			break
		}
		rem -= nb
	}
	lines := c.pendingLogs.lines[:n]
	c.pendingLogs.lines = c.pendingLogs.lines[n:]
	c.pendingLogs.Unlock()

	if len(lines) == 0 && !force {
		// Nothing to flush.
		return false
	}

	rescueLogs := false
	defer func() {
		if rescueLogs {
			c.pendingLogs.Lock()
			c.pendingLogs.lines = append(lines, c.pendingLogs.lines...)
			c.pendingLogs.Unlock()
		}
	}()

	buf, err := proto.Marshal(&logpb.UserAppLogGroup{
		LogLine: lines,
	})
	if err != nil {
		log.Printf("internal.flushLog: marshaling UserAppLogGroup: %v", err)
		rescueLogs = true
		return false
	}

	req := &logpb.FlushRequest{
		Logs: buf,
	}
	res := &basepb.VoidProto{}
	if err := c.Call("logservice", "Flush", req, res, nil); err != nil {
		log.Printf("internal.flushLog: Flush RPC: %v", err)
		rescueLogs = true
		return false
	}
	return true
}

const (
	// Log flushing parameters.
	flushInterval      = 1 * time.Second
	forceFlushInterval = 60 * time.Second
)

func (c *context) logFlusher(stop <-chan int) {
	lastFlush := time.Now()
	tick := time.NewTicker(flushInterval)
	for {
		select {
		case <-stop:
			// Request finished.
			tick.Stop()
			return
		case <-tick.C:
			force := time.Now().Sub(lastFlush) > forceFlushInterval
			if c.flushLog(force) {
				lastFlush = time.Now()
			}
		}
	}
}
