// Copyright 2018 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// +build go1.11

package internal

import (
	"log"

	netcontext "golang.org/x/net/context"
)

// DefaultVersionHostname needs to be implemented or deprecated. TODO(sbuss)
func DefaultVersionHostname(_ netcontext.Context) string {
	log.Println("appengine.DefaultVersionHostname() needs to be implemented or deprecated.")
	return ""
}

// Datacenter needs to be implemented or deprecated. TODO(sbuss)
func Datacenter(_ netcontext.Context) string {
	log.Println("appengine.Datacenter() needs to be implemented or deprecated.")
	return ""
}

// RequestID needs to be implemented or deprecated. TODO(sbuss)
func RequestID(_ netcontext.Context) string {
	log.Println("appengine.RequestID() needs to be implemented or deprecated.")
	return ""
}
