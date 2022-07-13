package internal

import (
	"os"
	"testing"
)

func TestIsDevAppServer(t *testing.T) {
	tests := []struct {
		desc string // See http://go/gotip/episodes/25 for naming guidance.
		env  map[string]string
		want bool
	}{
		{desc: "empty", env: map[string]string{}, want: false},
		{desc: "legacy", env: map[string]string{"RUN_WITH_DEVAPPSERVER": "1"}, want: true},
		{desc: "new", env: map[string]string{"GAE_ENV": "localdev"}, want: true},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			for key, value := range test.env {
				defer setenv(t, key, value)()
			}
			if got := IsDevAppServer(); got != test.want {
				t.Errorf("env=%v IsDevAppServer() got %v, want %v", test.env, got, test.want)
			}
		})
	}
}

// setenv is a backport of https://pkg.go.dev/testing#T.Setenv
func setenv(t *testing.T, key, value string) func() {
	t.Helper()
	prevValue, ok := os.LookupEnv(key)

	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("cannot set environment variable: %v", err)
	}

	if ok {
		return func() {
			os.Setenv(key, prevValue)
		}
	}
	return func() {
		os.Unsetenv(key)
	}
}
