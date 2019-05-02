// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// This change adds additional compatibility to help customers
// transition from google.golang.org/appengine/datastore to cloud.google.com/go/datastore.
// Each lib google.golang.org/appengine/datastore and cloud.google.com/go/datastore contain the functions Key.Encode() and Key.Decode(). These functions
// create base64 representations of a json marshalled type of datastore keys.  Customers have been using
// these encoded values to communicate between services in appengine.  The protobuf key types
// changed between google.golang.org/appengine/datastore and cloud.google.com/go/datastore making the corresponding base64 key strings incompatible.
// Customers who attempt to upgrade to cloud.google.com/go/datastore that use this pattern will fail.
// keycompatibility.go placed in google.golang.org/appengine/datastore enables forward compatibility of cloud.google.com/go/datastore encoded keys.
// An update to cloud.google.com/go/datastore will also be necessary to enable backward compatibility.

package datastore

import (
	"errors"
	"sync"

	"golang.org/x/net/context"

	"google.golang.org/appengine/datastore/internal/keycompat"
	"google.golang.org/appengine/internal"
)

var (
	errKeyConversion        = errors.New(`Key conversions must be enabled in the application. See https://github.com/golang/appengine#key-encode-decode-compatibiltiy-to-help-with-datastore-library-migrations for more details.`)
	keyConversionProject    string
	keyConversionEnableOnce sync.Once
)

// EnableKeyConversion enables forward key conversion abilities.  Calling this function in a single handler will enable
// the feature for all handlers.  This function can be called in the /_ah/start handler.
// The keyConversionProject variable holds the appid so that key conversion can retrieve it without a context.
func EnableKeyConversion(ctx context.Context) {
	keyConversionEnableOnce.Do(func() {
		keyConversionProject = internal.FullyQualifiedAppID(ctx)
	})
}

// convertNewKeyFormatToOldKeyFormat takes at new datastore key type and returns a old key type
func convertNewKeyFormatToOldKeyFormat(key *keycompat.NewFormatKey) (*Key, error) {
	// if key conversion is not enabled return right away
	if keyConversionProject == "" {
		return nil, errKeyConversion
	}
	var pKey *Key
	var err error
	if key.Parent != nil {
		pKey, err = convertNewKeyFormatToOldKeyFormat(key.Parent)
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
		appID:     keyConversionProject,
	}, nil
}
