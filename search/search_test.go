// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package search

import (
	"testing"
)

func TestValidIndexNameOrDocID(t *testing.T) {
	testCases := []struct {
		s    string
		want bool
	}{
		{"", true},
		{"!", false},
		{"$", true},
		{"!bad", false},
		{"good!", true},
		{"alsoGood", true},
		{"has spaces", false},
		{"is_inva\xffid_UTF-8", false},
		{"is_non-ASCïI", false},
		{"underscores_are_ok", true},
	}
	for _, tc := range testCases {
		if got := validIndexNameOrDocID(tc.s); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.s, got, tc.want)
		}
	}
}
