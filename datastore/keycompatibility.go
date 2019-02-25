// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// This change adds additional compatibility to help customers
// transition from google.golang.org/appengine/datastore (oldds) to cloud.google.com/go/datastore (newds).
// Each lib (oldds and newds) contain the functions Key.Encode() and Key.Decode(). These functions
// create base64 representations of a json marshalled type of datastore keys.  Customers have been using
// these encoded values to communicate between services in appengine.  The protobuf key types
// change between oldds and newds making the corresponding base64 key strings incompatible.
// Customers who attempt to upgrade to newds that use this pattern will fail.
// keycompatibility.go placed in oldds enables forward compatibility of newds encoded keys.
// An update to newds will also be necessary to enable backward compatibility.

package datastore

import (
	"errors"

	newds "cloud.google.com/go/datastore"
	"golang.org/x/net/context"
	"google.golang.org/appengine/internal"
)

var errKeyConversion = `Key conversions must be enabled in the application.
See https://github.com/golang/appengine#key-encode-decode-compatibiltiy-to-help-with-datastore-library-migrations for more details.`

var convKey *keyConverter

// EnableKeyConversion enables forward key conversion abilities.  Calling this function in a single handler will enable
// the feature for all handlers.  This function can be called in the /_ah/start handler.  Support for key converstion.
// Variable holds the appid so that key conversion can retrieve it without a context.
func EnableKeyConversion(ctx context.Context) {
	convKey = &keyConverter{
		appid: internal.FullyQualifiedAppID(ctx),
	}
	return
}

// keyConverter is the struct used to hold the appid for the process of key conversion
type keyConverter struct {
	appid string
}

// convertKey takes at new datastore key type and returns a old key type
func (c *keyConverter) convertNewKeyFormatToOldKeyFormat(key *newds.Key) (*Key, error) {
	// if key conversion is not enabled return right away
	if c == nil {
		return nil, errors.New(errKeyConversion)
	}
	var pKey *Key
	var err error
	if key.Parent != nil {
		pKey, err = c.convertNewKeyFormatToOldKeyFormat(key.Parent)
		if err != nil {
			return nil, err
		}
	}
	return &Key{
		intID:     key.ID,
		kind:      key.Kind,
		namespace: key.Namespace,
		parent:    pKey,
		stringID:  key.Name,
		appID:     c.appid,
	}, nil
}
