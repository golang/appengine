// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package internal

import (
	"net/http"
)

// These functions are implementations of the wrapper functions
// in ../appengine/identity.go. See that file for commentary.

const (
	hDefaultVersionHostname = "X-AppEngine-Default-Version-Hostname"
	hRequestLogId           = "X-AppEngine-Request-Log-Id"
	hDatacenter             = "X-AppEngine-Datacenter"
)

func DefaultVersionHostname(req interface{}) string {
	return req.(*http.Request).Header.Get(hDefaultVersionHostname)
}

func RequestID(req interface{}) string {
	return req.(*http.Request).Header.Get(hRequestLogId)
}

func Datacenter(req interface{}) string {
	return req.(*http.Request).Header.Get(hDatacenter)
}

func ServerSoftware() string {
	// TODO
	return "Google App Engine/1.x.x"
}

func ModuleName() string { return string(mustGetMetadata("instance/attributes/gae_backend_name")) }
func VersionID() string  { return string(mustGetMetadata("instance/attributes/gae_backend_version")) }
func InstanceID() string { return string(mustGetMetadata("instance/attributes/gae_backend_instance")) }
