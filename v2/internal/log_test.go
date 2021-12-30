// Copyright 2021 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestLogf(t *testing.T) {
	testCases := []struct {
		name          string
		deployed      bool
		level         int64
		format        string
		header        string
		args          []interface{}
		maxLogMessage int
		want          string
		wantJSON      bool
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
			name:          "local-long-lines-not-split",
			maxLogMessage: 10,
			format:        "0123456789a123",
			want:          "2021/05/12 16:09:52 DEBUG: 0123456789a123\n",
		},
		{
			name:     "deployed-plain-debug",
			deployed: true,
			level:    0,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message":"my abc 1","severity":"DEBUG"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-info",
			deployed: true,
			level:    1,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message":"my abc 1","severity":"INFO"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-warning",
			deployed: true,
			level:    2,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message":"my abc 1","severity":"WARNING"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-error",
			deployed: true,
			level:    3,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message":"my abc 1","severity":"ERROR"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-critical",
			deployed: true,
			level:    4,
			format:   "my %s %d",
			args:     []interface{}{"abc", 1},
			want:     `{"message":"my abc 1","severity":"CRITICAL"}` + "\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-multiline",
			deployed: true,
			level:    0,
			format:   "my \n multiline\n\n",
			want:     "{\"message\":\"my \\n multiline\\n\\n\",\"severity\":\"DEBUG\"}\n",
			wantJSON: true,
		},
		{
			name:     "deployed-plain-megaquote",
			deployed: true,
			level:    0,
			format:   `my "megaquote" %q`,
			args:     []interface{}{`internal "quote"`},
			want:     "{\"message\":\"my \\\"megaquote\\\" \\\"internal \\\\\\\"quote\\\\\\\"\\\"\",\"severity\":\"DEBUG\"}\n",
			wantJSON: true,
		},
		{
			name:          "deployed-too-long",
			deployed:      true,
			format:        "0123456789a123",
			maxLogMessage: 10,
			want:          "{\"message\":\"Part 1/2: 0123456789\",\"severity\":\"DEBUG\"}\n{\"message\":\"Part 2/2: a123\",\"severity\":\"DEBUG\"}\n",
		},
		{
			name:     "deployed-with-trace-header",
			deployed: true,
			format:   "my message",
			header:   "abc123/1234",
			want:     "{\"message\":\"my message\",\"severity\":\"DEBUG\",\"logging.googleapis.com/trace\":\"projects/my-project/traces/abc123\",\"logging.googleapis.com/spanId\":\"1234\"}\n",
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
		{
			name:          "deployed-structured-too-long",
			deployed:      true,
			format:        `{"message": "abc", "severity": "DEBUG"}`,
			maxLogMessage: 25,
			// User-structured logs must manually chunk; here we can see the structured message is (knowingly) broken.
			want: "Part 1/2: {\"message\": \"abc\", \"sever\nPart 2/2: ity\": \"DEBUG\"}\n",
		},
		{
			name:     "deployed-structured-with-trace-header",
			deployed: true,
			format:   `{"message": "abc", "severity": "DEBUG"}`,
			header:   "abc123/1234",
			want:     "{\"message\": \"abc\", \"severity\": \"DEBUG\"}\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := ""
			if tc.deployed {
				env = "standard"
			}
			defer setEnvVar(t, "GAE_ENV", env)()
			defer setEnvVar(t, "GOOGLE_CLOUD_PROJECT", "my-project")()
			var buf bytes.Buffer
			defer overrideLogStream(t, &buf)()
			defer overrideTimeNow(t, time.Date(2021, 5, 12, 16, 9, 52, 0, time.UTC))()
			if tc.maxLogMessage > 0 {
				defer overrideMaxLogMessage(t, tc.maxLogMessage)()
			}
			var headers []string
			if tc.header != "" {
				headers = []string{tc.header}
			}
			c := buildContextWithTraceHeaders(t, headers)

			logf(c, tc.level, tc.format, tc.args...)

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

func TestChunkLog(t *testing.T) {
	testCases := []struct {
		name string
		msg  string
		want []string
	}{
		{
			name: "empty",
			msg:  "",
			want: []string{""},
		},
		{
			name: "short",
			msg:  "short msg",
			want: []string{"short msg"},
		},
		{
			name: "exactly max",
			msg:  "0123456789",
			want: []string{"0123456789"},
		},
		{
			name: "too long",
			msg:  "0123456789a123",
			want: []string{
				"Part 1/2: 0123456789",
				"Part 2/2: a123",
			},
		},
		{
			name: "too long exactly max",
			msg:  "0123456789a123456789",
			want: []string{
				"Part 1/2: 0123456789",
				"Part 2/2: a123456789",
			},
		},
		{
			name: "longer",
			msg:  "0123456789a123456789b123456789c",
			want: []string{
				"Part 1/4: 0123456789",
				"Part 2/4: a123456789",
				"Part 3/4: b123456789",
				"Part 4/4: c",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer overrideMaxLogMessage(t, 10)()

			got := chunkLog(tc.msg)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("chunkLog() got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestTraceAndSpan(t *testing.T) {
	testCases := []struct {
		name        string
		header      []string
		wantTraceID string
		wantSpanID  string
	}{
		{
			name: "empty",
		},
		{
			name:   "header present, but empty",
			header: []string{""},
		},
		{
			name:        "trace and span",
			header:      []string{"abc1234/456"},
			wantTraceID: "projects/my-project/traces/abc1234",
			wantSpanID:  "456",
		},
		{
			name:        "trace and span with suffix",
			header:      []string{"abc1234/456;o=0"},
			wantTraceID: "projects/my-project/traces/abc1234",
			wantSpanID:  "456",
		},
		{
			name: "multiple headers, first taken",
			header: []string{
				"abc1234/456;o=1",
				"zzzzzzz/999;o=0",
			},
			wantTraceID: "projects/my-project/traces/abc1234",
			wantSpanID:  "456",
		},
		{
			name:   "missing trace",
			header: []string{"/456"},
		},
		{
			name:   "missing span",
			header: []string{"abc1234/"},
		},
		{
			name:   "random",
			header: []string{"somestring"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer setEnvVar(t, "GOOGLE_CLOUD_PROJECT", "my-project")()
			c := buildContextWithTraceHeaders(t, tc.header)

			gotTraceID, gotSpanID := traceAndSpan(c)

			if got, want := gotTraceID, tc.wantTraceID; got != want {
				t.Errorf("Incorrect traceID got=%q want=%q", got, want)
			}
			if got, want := gotSpanID, tc.wantSpanID; got != want {
				t.Errorf("Incorrect spanID got=%q want=%q", got, want)
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

func overrideMaxLogMessage(t *testing.T, max int) func() {
	t.Helper()
	old := maxLogMessage
	maxLogMessage = max
	return func() { maxLogMessage = old }
}

func buildContextWithTraceHeaders(t *testing.T, headers []string) *context {
	t.Helper()
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range headers {
		req.Header.Add("X-Cloud-Trace-Context", h)
	}
	return fromContext(ContextForTesting(req))
}
