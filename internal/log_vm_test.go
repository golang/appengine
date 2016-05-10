// Copyright 2016 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// +build !appengine

package internal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestJSONLogging(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	l := newJSONLogger(buf)

	c := &context{
		req: &http.Request{
			Header: http.Header{
				traceHeader: []string{"trace-id/more-data;o=1"},
			},
		},
		logger: l,
	}
	now := time.Now().Unix()

	logf(c, 0, "It's a lovely %s.", "day")

	// Log lines must be JSON encoded and end with a newline.
	if b := buf.Bytes()[buf.Len()-1]; b != '\n' {
		t.Error("log line missing trailing newline")
	}

	var line logLine
	if err := json.NewDecoder(buf).Decode(&line); err != nil {
		t.Fatalf("Failed to unmarshal log line: %v", err)
	}

	if got, want := line.Message, "It's a lovely day."; got != want {
		t.Errorf("line.Message = %q, want %s", got, want)
	}
	if got, want := line.Severity, "DEBUG"; got != want {
		t.Errorf("line.Severity = %s, want %s", got, want)
	}
	if got, want := line.TraceID, "trace-id"; got != want {
		t.Errorf("line.TraceID = %s, want %s", got, want)
	}
	if got := line.Timestamp.Seconds; got != now {
		t.Errorf("line.Timestamp.Seconds = %d, want %d", got, now)
	}
}
