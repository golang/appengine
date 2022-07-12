// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package delay

import (
	"bytes"
	stdctx "context"
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"

	"google.golang.org/appengine/v2/internal"
	"google.golang.org/appengine/v2/taskqueue"
)

type CustomType struct {
	N int
}

type CustomInterface interface {
	N() int
}

type CustomImpl int

func (c CustomImpl) N() int { return int(c) }

// CustomImpl needs to be registered with gob.
func init() {
	gob.Register(CustomImpl(0))
}

var (
	regFRuns = 0
	regFMsg  = ""
	regF     = func(c context.Context, arg string) {
		regFRuns++
		regFMsg = arg
	}
	regFunc     = Func("regFunc", regF)
	regRegister = MustRegister("regRegister", regF)

	custFTally = 0
	custF      = func(c context.Context, ct *CustomType, ci CustomInterface) {
		a, b := 2, 3
		if ct != nil {
			a = ct.N
		}
		if ci != nil {
			b = ci.N()
		}
		custFTally += a + b
	}
	custFunc     = Func("custFunc", custF)
	custRegister = MustRegister("custRegister", custF)

	anotherCustFunc = Func("custFunc2", func(c context.Context, n int, ct *CustomType, ci CustomInterface) {
	})

	varFMsg = ""
	varF    = func(c context.Context, format string, args ...int) {
		// convert []int to []interface{} for fmt.Sprintf.
		as := make([]interface{}, len(args))
		for i, a := range args {
			as[i] = a
		}
		varFMsg = fmt.Sprintf(format, as...)
	}
	varFunc     = Func("variadicFunc", varF)
	varRegister = MustRegister("variadicRegister", varF)

	errFRuns = 0
	errFErr  = errors.New("error!")
	errF     = func(c context.Context) error {
		errFRuns++
		if errFRuns == 1 {
			return nil
		}
		return errFErr
	}
	errFunc     = Func("errFunc", errF)
	errRegister = MustRegister("errRegister", errF)

	dupeWhich = 0
	dupe1F    = func(c context.Context) {
		if dupeWhich == 0 {
			dupeWhich = 1
		}
	}
	dupe1Func = Func("dupe", dupe1F)
	dupe2F    = func(c context.Context) {
		if dupeWhich == 0 {
			dupeWhich = 2
		}
	}
	dupe2Func = Func("dupe", dupe2F)

	requestFuncRuns    = 0
	requestFuncHeaders *taskqueue.RequestHeaders
	requestFuncErr     error
	requestF           = func(c context.Context) {
		requestFuncRuns++
		requestFuncHeaders, requestFuncErr = RequestHeaders(c)
	}
	requestFunc     = Func("requestFunc", requestF)
	requestRegister = MustRegister("requestRegister", requestF)

	stdCtxRuns = 0
	stdCtxF    = func(c stdctx.Context) {
		stdCtxRuns++
	}
	stdCtxFunc     = Func("stdctxFunc", stdCtxF)
	stdCtxRegister = MustRegister("stdctxRegister", stdCtxF)
)

type fakeContext struct {
	ctx     context.Context
	logging [][]interface{}
}

func newFakeContext() *fakeContext {
	f := new(fakeContext)
	f.ctx = internal.WithCallOverride(context.Background(), f.call)
	f.ctx = internal.WithLogOverride(f.ctx, f.logf)
	return f
}

func (f *fakeContext) call(ctx context.Context, service, method string, in, out proto.Message) error {
	panic("should never be called")
}

var logLevels = map[int64]string{1: "INFO", 3: "ERROR"}

func (f *fakeContext) logf(level int64, format string, args ...interface{}) {
	f.logging = append(f.logging, append([]interface{}{logLevels[level], format}, args...))
}

func TestInvalidFunction(t *testing.T) {
	c := newFakeContext()
	invalidFunc := Func("invalid", func() {})

	if got, want := invalidFunc.Call(c.ctx), fmt.Errorf("delay: func is invalid: %s", errFirstArg); got.Error() != want.Error() {
		t.Errorf("Incorrect error: got %q, want %q", got, want)
	}
}

func TestVariadicFunctionArguments(t *testing.T) {
	// Check the argument type validation for variadic functions.
	c := newFakeContext()

	calls := 0
	taskqueueAdder = func(c context.Context, t *taskqueue.Task, _ string) (*taskqueue.Task, error) {
		calls++
		return t, nil
	}

	for _, testTarget := range []*Function{varFunc, varRegister} {
		// reset state
		calls = 0
		testTarget.Call(c.ctx, "hi")
		testTarget.Call(c.ctx, "%d", 12)
		testTarget.Call(c.ctx, "%d %d %d", 3, 1, 4)
		if calls != 3 {
			t.Errorf("Got %d calls to taskqueueAdder, want 3", calls)
		}

		if got, want := testTarget.Call(c.ctx, "%d %s", 12, "a string is bad"), errors.New("delay: argument 3 has wrong type: string is not assignable to int"); got.Error() != want.Error() {
			t.Errorf("Incorrect error: got %q, want %q", got, want)
		}
	}
}

func TestBadArguments(t *testing.T) {
	// Try running regFunc with different sets of inappropriate arguments.

	c := newFakeContext()

	tests := []struct {
		args    []interface{} // all except context
		wantErr string
	}{
		{
			args:    nil,
			wantErr: "delay: too few arguments to func: 1 < 2",
		},
		{
			args:    []interface{}{"lala", 53},
			wantErr: "delay: too many arguments to func: 3 > 2",
		},
		{
			args:    []interface{}{53},
			wantErr: "delay: argument 1 has wrong type: int is not assignable to string",
		},
	}
	for _, testTarget := range []*Function{regFunc, regRegister} {
		for i, tc := range tests {
			got := testTarget.Call(c.ctx, tc.args...)
			if got.Error() != tc.wantErr {
				t.Errorf("Call %v: got %q, want %q", i, got, tc.wantErr)
			}
		}
	}
}

func TestRunningFunction(t *testing.T) {
	c := newFakeContext()
	// Fake out the adding of a task.
	var task *taskqueue.Task
	taskqueueAdder = func(_ context.Context, tk *taskqueue.Task, queue string) (*taskqueue.Task, error) {
		if queue != "" {
			t.Errorf(`Got queue %q, expected ""`, queue)
		}
		task = tk
		return tk, nil
	}

	for _, testTarget := range []*Function{regFunc, regRegister} {
		regFRuns, regFMsg = 0, "" // reset state
		const msg = "Why, hello!"
		testTarget.Call(c.ctx, msg)

		// Simulate the Task Queue service.
		req, err := http.NewRequest("POST", path, bytes.NewBuffer(task.Payload))
		if err != nil {
			t.Fatalf("Failed making http.Request: %v", err)
		}
		rw := httptest.NewRecorder()
		runFunc(c.ctx, rw, req)

		if regFRuns != 1 {
			t.Errorf("regFuncRuns: got %d, want 1", regFRuns)
		}
		if regFMsg != msg {
			t.Errorf("regFuncMsg: got %q, want %q", regFMsg, msg)
		}
	}
}

func TestCustomType(t *testing.T) {
	c := newFakeContext()

	// Fake out the adding of a task.
	var task *taskqueue.Task
	taskqueueAdder = func(_ context.Context, tk *taskqueue.Task, queue string) (*taskqueue.Task, error) {
		if queue != "" {
			t.Errorf(`Got queue %q, expected ""`, queue)
		}
		task = tk
		return tk, nil
	}

	for _, testTarget := range []*Function{custFunc, custRegister} {
		custFTally = 0 // reset state
		testTarget.Call(c.ctx, &CustomType{N: 11}, CustomImpl(13))

		// Simulate the Task Queue service.
		req, err := http.NewRequest("POST", path, bytes.NewBuffer(task.Payload))
		if err != nil {
			t.Fatalf("Failed making http.Request: %v", err)
		}
		rw := httptest.NewRecorder()
		runFunc(c.ctx, rw, req)

		if custFTally != 24 {
			t.Errorf("custFTally = %d, want 24", custFTally)
		}

		// Try the same, but with nil values; one is a nil pointer (and thus a non-nil interface value),
		// and the other is a nil interface value.
		custFTally = 0 // reset state
		testTarget.Call(c.ctx, (*CustomType)(nil), nil)

		// Simulate the Task Queue service.
		req, err = http.NewRequest("POST", path, bytes.NewBuffer(task.Payload))
		if err != nil {
			t.Fatalf("Failed making http.Request: %v", err)
		}
		rw = httptest.NewRecorder()
		runFunc(c.ctx, rw, req)

		if custFTally != 5 {
			t.Errorf("custFTally = %d, want 5", custFTally)
		}
	}
}

func TestRunningVariadic(t *testing.T) {
	c := newFakeContext()

	// Fake out the adding of a task.
	var task *taskqueue.Task
	taskqueueAdder = func(_ context.Context, tk *taskqueue.Task, queue string) (*taskqueue.Task, error) {
		if queue != "" {
			t.Errorf(`Got queue %q, expected ""`, queue)
		}
		task = tk
		return tk, nil
	}

	for _, testTarget := range []*Function{varFunc, varRegister} {
		varFMsg = "" // reset state
		testTarget.Call(c.ctx, "Amiga %d has %d KB RAM", 500, 512)

		// Simulate the Task Queue service.
		req, err := http.NewRequest("POST", path, bytes.NewBuffer(task.Payload))
		if err != nil {
			t.Fatalf("Failed making http.Request: %v", err)
		}
		rw := httptest.NewRecorder()
		runFunc(c.ctx, rw, req)

		const expected = "Amiga 500 has 512 KB RAM"
		if varFMsg != expected {
			t.Errorf("varFMsg = %q, want %q", varFMsg, expected)
		}
	}
}

func TestErrorFunction(t *testing.T) {
	c := newFakeContext()

	// Fake out the adding of a task.
	var task *taskqueue.Task
	taskqueueAdder = func(_ context.Context, tk *taskqueue.Task, queue string) (*taskqueue.Task, error) {
		if queue != "" {
			t.Errorf(`Got queue %q, expected ""`, queue)
		}
		task = tk
		return tk, nil
	}

	for _, testTarget := range []*Function{errFunc, errRegister} {
		// reset state
		c.logging = [][]interface{}{}
		errFRuns = 0
		testTarget.Call(c.ctx)

		// Simulate the Task Queue service.
		// The first call should succeed; the second call should fail.
		{
			req, err := http.NewRequest("POST", path, bytes.NewBuffer(task.Payload))
			if err != nil {
				t.Fatalf("Failed making http.Request: %v", err)
			}
			rw := httptest.NewRecorder()
			runFunc(c.ctx, rw, req)
		}
		{
			req, err := http.NewRequest("POST", path, bytes.NewBuffer(task.Payload))
			if err != nil {
				t.Fatalf("Failed making http.Request: %v", err)
			}
			rw := httptest.NewRecorder()
			runFunc(c.ctx, rw, req)
			if rw.Code != http.StatusInternalServerError {
				t.Errorf("Got status code %d, want %d", rw.Code, http.StatusInternalServerError)
			}

			wantLogging := [][]interface{}{
				{"ERROR", "delay: func failed (will retry): %v", errFErr},
			}
			if !reflect.DeepEqual(c.logging, wantLogging) {
				t.Errorf("Incorrect logging: got %+v, want %+v", c.logging, wantLogging)
			}
		}
	}
}

func TestFuncDuplicateFunction(t *testing.T) {
	c := newFakeContext()

	// Fake out the adding of a task.
	var task *taskqueue.Task
	taskqueueAdder = func(_ context.Context, tk *taskqueue.Task, queue string) (*taskqueue.Task, error) {
		if queue != "" {
			t.Errorf(`Got queue %q, expected ""`, queue)
		}
		task = tk
		return tk, nil
	}

	if err := dupe1Func.Call(c.ctx); err == nil {
		t.Error("dupe1Func.Call did not return error")
	}
	if task != nil {
		t.Error("dupe1Func.Call posted a task")
	}
	if err := dupe2Func.Call(c.ctx); err != nil {
		t.Errorf("dupe2Func.Call error: %v", err)
	}
	if task == nil {
		t.Fatalf("dupe2Func.Call did not post a task")
	}

	// Simulate the Task Queue service.
	req, err := http.NewRequest("POST", path, bytes.NewBuffer(task.Payload))
	if err != nil {
		t.Fatalf("Failed making http.Request: %v", err)
	}
	rw := httptest.NewRecorder()
	runFunc(c.ctx, rw, req)

	if dupeWhich == 1 {
		t.Error("dupe2Func.Call used old registered function")
	} else if dupeWhich != 2 {
		t.Errorf("dupeWhich = %d; want 2", dupeWhich)
	}
}

func TestMustRegisterDuplicateFunction(t *testing.T) {
	MustRegister("dupe", dupe1F)
	defer func() {
		err := recover()
		if err == nil {
			t.Error("MustRegister did not panic")
		}
		got := fmt.Sprintf("%s", err)
		want := fmt.Sprintf("multiple functions registered for %q", "dupe")
		if got != want {
			t.Errorf("Incorrect error: got %q, want %q", got, want)
		}
	}()
	MustRegister("dupe", dupe2F)
}

func TestInvalidFunction_MustRegister(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Error("MustRegister did not panic")
		}
		if err != errFirstArg {
			t.Errorf("Incorrect error: got %q, want %q", err, errFirstArg)
		}
	}()
	MustRegister("invalid", func() {})
}

func TestGetRequestHeadersFromContext(t *testing.T) {
	for _, testTarget := range []*Function{requestFunc, requestRegister} {
		c := newFakeContext()

		// Outside a delay.Func should return an error.
		headers, err := RequestHeaders(c.ctx)
		if headers != nil {
			t.Errorf("RequestHeaders outside Func, got %v, want nil", headers)
		}
		if err != errOutsideDelayFunc {
			t.Errorf("RequestHeaders outside Func err, got %v, want %v", err, errOutsideDelayFunc)
		}

		// Fake out the adding of a task.
		var task *taskqueue.Task
		taskqueueAdder = func(_ context.Context, tk *taskqueue.Task, queue string) (*taskqueue.Task, error) {
			if queue != "" {
				t.Errorf(`Got queue %q, expected ""`, queue)
			}
			task = tk
			return tk, nil
		}

		testTarget.Call(c.ctx)

		requestFuncRuns, requestFuncHeaders = 0, nil // reset state
		// Simulate the Task Queue service.
		req, err := http.NewRequest("POST", path, bytes.NewBuffer(task.Payload))
		req.Header.Set("x-appengine-taskname", "foobar")
		if err != nil {
			t.Fatalf("Failed making http.Request: %v", err)
		}
		rw := httptest.NewRecorder()
		runFunc(c.ctx, rw, req)

		if requestFuncRuns != 1 {
			t.Errorf("requestFuncRuns: got %d, want 1", requestFuncRuns)
		}
		if requestFuncHeaders.TaskName != "foobar" {
			t.Errorf("requestFuncHeaders.TaskName: got %v, want 'foobar'", requestFuncHeaders.TaskName)
		}
		if requestFuncErr != nil {
			t.Errorf("requestFuncErr: got %v, want nil", requestFuncErr)
		}
	}
}

func TestStandardContext(t *testing.T) {
	// Fake out the adding of a task.
	var task *taskqueue.Task
	taskqueueAdder = func(_ context.Context, tk *taskqueue.Task, queue string) (*taskqueue.Task, error) {
		if queue != "" {
			t.Errorf(`Got queue %q, expected ""`, queue)
		}
		task = tk
		return tk, nil
	}

	for _, testTarget := range []*Function{stdCtxFunc, stdCtxRegister} {
		c := newFakeContext()
		stdCtxRuns = 0 // reset state
		if err := testTarget.Call(c.ctx); err != nil {
			t.Fatal("Function.Call:", err)
		}

		// Simulate the Task Queue service.
		req, err := http.NewRequest("POST", path, bytes.NewBuffer(task.Payload))
		if err != nil {
			t.Fatalf("Failed making http.Request: %v", err)
		}
		rw := httptest.NewRecorder()
		runFunc(c.ctx, rw, req)

		if stdCtxRuns != 1 {
			t.Errorf("stdCtxRuns: got %d, want 1", stdCtxRuns)
		}
	}
}

func TestFileKey(t *testing.T) {
	const firstGenTest = 0
	tests := []struct {
		mainPath string
		file     string
		want     string
	}{
		// first-gen
		{
			"",
			filepath.FromSlash("srv/foo.go"),
			filepath.FromSlash("srv/foo.go"),
		},
		// gopath
		{
			filepath.FromSlash("/tmp/staging1234/srv/"),
			filepath.FromSlash("/tmp/staging1234/srv/foo.go"),
			"foo.go",
		},
		{
			filepath.FromSlash("/tmp/staging1234/srv/_gopath/src/example.com/foo"),
			filepath.FromSlash("/tmp/staging1234/srv/_gopath/src/example.com/foo/foo.go"),
			"foo.go",
		},
		{
			filepath.FromSlash("/tmp/staging2234/srv/_gopath/src/example.com/foo"),
			filepath.FromSlash("/tmp/staging2234/srv/_gopath/src/example.com/foo/bar/bar.go"),
			filepath.FromSlash("example.com/foo/bar/bar.go"),
		},
		{
			filepath.FromSlash("/tmp/staging3234/srv/_gopath/src/example.com/foo"),
			filepath.FromSlash("/tmp/staging3234/srv/_gopath/src/example.com/bar/main.go"),
			filepath.FromSlash("example.com/bar/main.go"),
		},
		{
			filepath.FromSlash("/tmp/staging3234/srv/gopath/src/example.com/foo"),
			filepath.FromSlash("/tmp/staging3234/srv/gopath/src/example.com/bar/main.go"),
			filepath.FromSlash("example.com/bar/main.go"),
		},
		{
			filepath.FromSlash(""),
			filepath.FromSlash("/tmp/staging3234/srv/gopath/src/example.com/bar/main.go"),
			filepath.FromSlash("example.com/bar/main.go"),
		},
		// go mod, same package
		{
			filepath.FromSlash("/tmp/staging3234/srv"),
			filepath.FromSlash("/tmp/staging3234/srv/main.go"),
			"main.go",
		},
		{
			filepath.FromSlash("/tmp/staging3234/srv"),
			filepath.FromSlash("/tmp/staging3234/srv/bar/main.go"),
			filepath.FromSlash("bar/main.go"),
		},
		{
			filepath.FromSlash("/tmp/staging3234/srv/cmd"),
			filepath.FromSlash("/tmp/staging3234/srv/cmd/main.go"),
			"main.go",
		},
		{
			filepath.FromSlash("/tmp/staging3234/srv/cmd"),
			filepath.FromSlash("/tmp/staging3234/srv/bar/main.go"),
			filepath.FromSlash("bar/main.go"),
		},
		{
			filepath.FromSlash(""),
			filepath.FromSlash("/tmp/staging3234/srv/bar/main.go"),
			filepath.FromSlash("bar/main.go"),
		},
		// go mod, other package
		{
			filepath.FromSlash("/tmp/staging3234/srv"),
			filepath.FromSlash("/go/pkg/mod/github.com/foo/bar@v0.0.0-20181026220418-f595d03440dc/baz.go"),
			filepath.FromSlash("github.com/foo/bar/baz.go"),
		},
	}
	for i, tc := range tests {
		if i > firstGenTest {
			os.Setenv("GAE_ENV", "standard")
		}
		internal.MainPath = tc.mainPath
		got, err := fileKey(tc.file)
		if err != nil {
			t.Errorf("Unexpected error, call %v, file %q: %v", i, tc.file, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Call %v, file %q: got %q, want %q", i, tc.file, got, tc.want)
		}
	}
}
