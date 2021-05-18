// Copyright 2021 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

func TestLogf(t *testing.T) {
	testCases := []struct {
		name     string
		deployed bool
		level    int64
		format   string
		args     []interface{}
		want     string
		wantJSON bool
	}{
		{
			name:   "local-debug",
			level:  0,
			format: "my %s %d",
			args:   []interface{}{"abc", 1},
			want:   "2021/05/12 16:09:52 DEBUG: my abc 1\n",
		},
		{
			name:   "local-info",
			level:  1,
			format: "my %s %d",
			args:   []interface{}{"abc", 1},
			want:   "2021/05/12 16:09:52 INFO: my abc 1\n",
		},
		{
			name:   "local-warning",
			level:  2,
			format: "my %s %d",
			args:   []interface{}{"abc", 1},
			want:   "2021/05/12 16:09:52 WARNING: my abc 1\n",
		},
		{
			name:   "local-error",
			level:  3,
			format: "my %s %d",
			args:   []interface{}{"abc", 1},
			want:   "2021/05/12 16:09:52 ERROR: my abc 1\n",
		},
		{
			name:   "local-critical",
			level:  4,
			format: "my %s %d",
			args:   []interface{}{"abc", 1},
			want:   "2021/05/12 16:09:52 CRITICAL: my abc 1\n",
		},
		{
			name:   "local-multiline",
			level:  0,
			format: "my \n multiline\n\n",
			want:   "2021/05/12 16:09:52 DEBUG: my \n multiline\n",
		},
		{
			name:     "deployed-plain-debug",
			deployed: true,
			level:    0,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message": "my abc 1", "severity": "DEBUG"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-info",
			deployed: true,
			level:    1,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message": "my abc 1", "severity": "INFO"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-warning",
			deployed: true,
			level:    2,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message": "my abc 1", "severity": "WARNING"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-error",
			deployed: true,
			level:    3,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message": "my abc 1", "severity": "ERROR"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-critical",
			deployed: true,
			level:    4,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message": "my abc 1", "severity": "CRITICAL"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-multiline",
			deployed: true,
			level:    0,
			format:   "my \n multiline\n\n",
			want:     "{\"message\": \"my \\n multiline\\n\\n\", \"severity\": \"DEBUG\"}\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-megaquote",
			deployed: true,
			level:    0,
			format:   `my "megaquote" %q`,
			args:     []interface{}{`internal "quote"`},
			want:     "{\"message\": \"my \\\"megaquote\\\" \\\"internal \\\\\\\"quote\\\\\\\"\\\"\", \"severity\": \"DEBUG\"}\n",
			wantJSON: true,
		},
		{
			name:     "deployed-structured-debug",
			deployed: true,
			level:    0,
			format:   `{"some": "message %s %d"}`,
			args:     []interface{}{"abc", 1},
			want:     `{"some": "message abc 1"}` + "\n",
		},
		{
			name:     "deployed-structured-info",
			deployed: true,
			level:    1,
			format:   `{"some": "message %s %d"}`,
			args:     []interface{}{"abc", 1},
			want:     `{"some": "message abc 1"}` + "\n",
		},
		{
			name:     "deployed-structured-warning",
			deployed: true,
			level:    2,
			format:   `{"some": "message %s %d"}`,
			args:     []interface{}{"abc", 1},
			want:     `{"some": "message abc 1"}` + "\n",
		},
		{
			name:     "deployed-structured-error",
			deployed: true,
			level:    3,
			format:   `{"some": "message %s %d"}`,
			args:     []interface{}{"abc", 1},
			want:     `{"some": "message abc 1"}` + "\n",
		},
		{
			name:     "deployed-structured-critical",
			deployed: true,
			level:    4,
			format:   `{"some": "message %s %d"}`,
			args:     []interface{}{"abc", 1},
			want:     `{"some": "message abc 1"}` + "\n",
		},
		{
			// The leading "{" assumes this is already a structured log, so no alteration is performed.
			name:     "deployed-structured-multiline",
			deployed: true,
			level:    4,
			// This is not even valid JSON; we don't attempt to validate and only use the first character.
			format: "{\"some\": \"message\n%s %d\"",
			args:   []interface{}{"abc", 1},
			want:   "{\"some\": \"message\nabc 1\"\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := ""
			if tc.deployed {
				env = "standard"
			}
			defer setEnvVar(t, "GAE_ENV", env)()
			var buf bytes.Buffer
			defer overrideLogStream(t, &buf)()
			defer overrideTimeNow(t, time.Date(2021, 5, 12, 16, 9, 52, 0, time.UTC))()
			ctx := fromContext(BackgroundContext())

			logf(ctx, tc.level, tc.format, tc.args...)

			if got, want := buf.String(), tc.want; got != want {
				t.Errorf("incorrect log got=%q want=%q", got, want)
			}

			if tc.wantJSON {
				var e struct {
					Message  string `json:"message"`
					Severity string `json:"severity"`
				}
				if err := json.Unmarshal(buf.Bytes(), &e); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if gotMsg, wantMsg := e.Message, fmt.Sprintf(tc.format, tc.args...); gotMsg != wantMsg {
					t.Errorf("JSON-encoded message incorrect got=%q want=%q", gotMsg, wantMsg)
				}
				if gotSev, wantSev := e.Severity, logLevelName[tc.level]; gotSev != wantSev {
					t.Errorf("JSON-encoded severity incorrect got=%q want=%q", gotSev, wantSev)
				}
			}
		})
	}
}

func setEnvVar(t *testing.T, key, value string) func() {
	t.Helper()
	old, present := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatal(err)
	}
	return func() {
		if present {
			if err := os.Setenv(key, old); err != nil {
				t.Fatal(err)
			}
			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func overrideLogStream(t *testing.T, writer io.Writer) func() {
	t.Helper()
	old := logStream
	logStream = writer
	return func() { logStream = old }
}

func overrideTimeNow(t *testing.T, now time.Time) func() {
	t.Helper()
	old := timeNow
	timeNow = func() time.Time { return now }
	return func() { timeNow = old }
}
