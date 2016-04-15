// Copyright 2016 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

func init() {
	addTestCases(aeTests, nil)
}

var aeTests = []testCase{
	// Collection of fixes:
	//	- imports
	//	- appengine.Timeout -> context.WithTimeout
	//	- add ctx arg to appengine.Datacenter
	//	- logging API
	{
		Name: "ae.0",
		In: `package foo

import (
	"net/http"
	"time"

	"appengine"
	"appengine/datastore"
)

func f(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	c = appengine.Timeout(c, 5*time.Second)
	err := datastore.ErrNoSuchEntity
	c.Errorf("Something interesting happened: %v", err)
	_ = appengine.Datacenter()
}
`,
		Out: `package foo

import (
	"net/http"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

func f(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	c, _ = context.WithTimeout(c, 5*time.Second)
	err := datastore.ErrNoSuchEntity
	log.Errorf(c, "Something interesting happened: %v", err)
	_ = appengine.Datacenter(c)
}
`,
	},
}
