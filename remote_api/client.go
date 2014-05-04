// Copyright 2013 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package remote_api

// This file provides the client for connecting remotely to a user's production
// application.

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"code.google.com/p/goprotobuf/proto"

	"google.golang.org/appengine"
	"google.golang.org/appengine/internal"
	pb "google.golang.org/appengine/internal/remote_api"
)

// NewRemoteContext returns a context that gives access to the production
// APIs for the application at the given host. All communication will be
// performed over SSL unless the host is localhost.
func NewRemoteContext(host string, client *http.Client) (appengine.Context, error) {
	// Add an appcfg header to outgoing requests.
	t := client.Transport
	if t == nil {
		t = http.DefaultTransport
	}
	client.Transport = &headerAddingRoundTripper{t}

	url := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/_ah/remote_api",
	}
	if regexp.MustCompile(`^localhost(:\d{1,5})?$`).MatchString(host) {
		url.Scheme = "http"
	}
	u := url.String()
	appID, err := getAppID(client, u)
	if err != nil {
		return nil, fmt.Errorf("unable to contact server: %v", err)
	}
	return &context{
		client: client,
		url:    u,
		appID:  appID,
	}, nil
}

type context struct {
	client *http.Client
	url    string
	appID  string
}

func (c *context) Request() interface{}        { return nil }
func (c *context) FullyQualifiedAppID() string { return c.appID }

func (c *context) logf(level, format string, args ...interface{}) {
	log.Printf(level+": "+format, args...)
}

func (c *context) Debugf(format string, args ...interface{})    { c.logf("DEBUG", format, args...) }
func (c *context) Infof(format string, args ...interface{})     { c.logf("INFO", format, args...) }
func (c *context) Warningf(format string, args ...interface{})  { c.logf("WARNING", format, args...) }
func (c *context) Errorf(format string, args ...interface{})    { c.logf("ERROR", format, args...) }
func (c *context) Criticalf(format string, args ...interface{}) { c.logf("CRITICAL", format, args...) }

func (c *context) Call(service, method string, in, out proto.Message, opts *internal.CallOptions) error {
	req, err := proto.Marshal(in)
	if err != nil {
		return fmt.Errorf("error marshalling request: %v", err)
	}

	remReq := &pb.Request{
		ServiceName: proto.String(service),
		Method:      proto.String(method),
		Request:     req,
		// NOTE(djd): RequestId is unused in the server.
	}

	req, err = proto.Marshal(remReq)
	if err != nil {
		return fmt.Errorf("proto.Marshal: %v", err)
	}

	// TODO(djd): Respect opts.Timeout?
	resp, err := c.client.Post(c.url, "application/octet-stream", bytes.NewReader(req))
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad response %d; body: %q", resp.StatusCode, body)
	}
	if err != nil {
		return fmt.Errorf("failed reading response: %v", err)
	}
	remResp := &pb.Response{}
	if err := proto.Unmarshal(body, remResp); err != nil {
		return fmt.Errorf("error unmarshalling response: %v", err)
	}

	if ae := remResp.GetApplicationError(); ae != nil {
		return &internal.APIError{
			Code:    ae.GetCode(),
			Detail:  ae.GetDetail(),
			Service: service,
		}
	}

	if remResp.Response == nil {
		return fmt.Errorf("unexpected response: %s", proto.MarshalTextString(remResp))
	}

	return proto.Unmarshal(remResp.Response, out)
}

// This is a forgiving regexp designed to parse the app ID from YAML.
var appIDRE = regexp.MustCompile(`app_id["']?\s*:\s*['"?]([-a-z0-9.:~]+)`)

func getAppID(client *http.Client, url string) (string, error) {
	// Generate a pseudo-random token for handshaking.
	token := strconv.Itoa(rand.New(rand.NewSource(time.Now().UnixNano())).Int())

	resp, err := client.Get(fmt.Sprintf("%s?rtok=%s", url, token))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad response %d; body: %q", resp.StatusCode, body)
	}
	if err != nil {
		return "", fmt.Errorf("failed reading response: %v", err)
	}

	// Check the token is present in response.
	if !bytes.Contains(body, []byte(token)) {
		return "", fmt.Errorf("token not found: want %q; body %q", token, body)
	}

	match := appIDRE.FindSubmatch(body)
	if match == nil {
		return "", fmt.Errorf("app ID not found: body %q", body)
	}

	return string(match[1]), nil
}

type headerAddingRoundTripper struct {
	Wrapped http.RoundTripper
}

func (t *headerAddingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("X-Appcfg-Api-Version", "1")
	return t.Wrapped.RoundTrip(r)
}
