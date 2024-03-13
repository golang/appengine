// Copyright 2019 Google Inc. All Rights Reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package datastore

import (
	"reflect"
	"testing"
)

func TestKeyConversion(t *testing.T) {
	var tests = []struct {
		desc       string
		key        *Key
		encodedKey string
	}{

		{
			desc: "A control test for legacy to legacy key conversion int as the key",
			key: &Key{
				kind:  "Person",
				intID: 1,
				appID: "glibrary",
			},
			encodedKey: "aghnbGlicmFyeXIMCgoSBlBlcnNvbhgB",
		},
		{
			desc: "A control test for legacy to legacy key conversion string as the key",
			key: &Key{
				kind:     "Graph",
				stringID: "graph:7-day-active",
				appID:    "glibrary",
			},
			encodedKey: "aghnbGlicmFyeXIdChsSBUdyYXBoIhJncmFwaDo3LWRheS1hY3RpdmU",
		},

		// These are keys encoded with cloud.google.com/go/datastore
		// Standard int as the key
		{
			desc: "Convert new key format to old key with int id",
			key: &Key{
				kind:  "WordIndex",
				intID: 1033,
				appID: "glibrary",
			},
			encodedKey: "aghnbGlicmFyeXIQCg4SCVdvcmRJbmRleBiJCA",
		},
		// Standard string
		{
			desc: "Convert new key format to old key with string id",
			key: &Key{
				kind:     "WordIndex",
				stringID: "IAmAnID",
				appID:    "glibrary",
			},
			encodedKey: "aghnbGlicmFyeXIWChQSCVdvcmRJbmRleCIHSUFtQW5JRA",
		},

		// These are keys encoded with cloud.google.com/go/datastore
		// ID String with parent as string
		{
			desc: "Convert new key format to old key with string id with a parent",
			key: &Key{
				kind:     "WordIndex",
				stringID: "IAmAnID",
				appID:    "glibrary",
				parent: &Key{
					kind:     "LetterIndex",
					stringID: "IAmAnotherID",
					appID:    "glibrary",
				},
			},
			encodedKey: "aghnbGlicmFyeXIzChsSC0xldHRlckluZGV4IgxJQW1Bbm90aGVySUQKFBIJV29yZEluZGV4IgdJQW1BbklE",
		},
	}

	// Simulate the key converter enablement
	keyConversion.appID = "glibrary"
	for _, tc := range tests {
		enc := tc.key.Encode()
		dk, err := DecodeKey(tc.encodedKey)
		if err != nil {
			t.Fatalf("DecodeKey: %v %s", err, enc)
		}
		if !reflect.DeepEqual(dk, tc.key) {
			t.Errorf("%s: got %+v, want %+v %s", tc.desc, dk, tc.key, enc)
		}
	}
}
