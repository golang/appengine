// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

/*
Package delay provides a way to execute code outside of the scope of
a user request by using the Task Queue API.
To use a deferred function, you must register the function to be
deferred as a top-level var. For example,
    ```
    var laterFunc = delay.MustRegister("key", myFunc)
    func myFunc(ctx context.Context, a, b string) {...}
    ```
You can also inline with a function literal:
    ```
    var laterFunc = delay.MustRegister("key", func(ctx context.Context, a, b string) {...})
    ```
In the above example, "key" is a logical name for the function.
The key needs to be globally unique across your entire application.
To invoke the function in a deferred fashion, call the top-level item:
    ```
    laterFunc(ctx, "aaa", "bbb")
    ```

This will queue a task and return quickly; the function will be actually
run in a new request. The delay package uses the Task Queue API to create
tasks that call the reserved application path "/_ah/queue/go/delay".
This path may only be marked as "login: admin" or have no access
restriction; it will fail if marked as "login: required".
*/

package delay // import "google.golang.org/appengine/v2/delay"

import (
	"bytes"
	stdctx "context"
	"encoding/gob"
	"errors"
	"fmt"
	"go/build"
	stdlog "log"
	"net/http"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/net/context"

	"google.golang.org/appengine/v2"
	"google.golang.org/appengine/v2/internal"
	"google.golang.org/appengine/v2/log"
	"google.golang.org/appengine/v2/taskqueue"
)

// Function represents a function that may have a delayed invocation.
type Function struct {
	fv  reflect.Value // Kind() == reflect.Func
	key string
	err error // any error during initialization
}

const (
	// The HTTP path for invocations.
	path = "/_ah/queue/go/delay"
	// Use the default queue.
	queue = ""
)

type contextKey int

var (
	// registry of all delayed functions
	funcs = make(map[string]*Function)

	// precomputed types
	errorType = reflect.TypeOf((*error)(nil)).Elem()

	// errors
	errFirstArg         = errors.New("first argument must be context.Context")
	errOutsideDelayFunc = errors.New("request headers are only available inside a delay.Func")

	// context keys
	headersContextKey contextKey = 0
	stdContextType               = reflect.TypeOf((*stdctx.Context)(nil)).Elem()
	netContextType               = reflect.TypeOf((*context.Context)(nil)).Elem()
)

func isContext(t reflect.Type) bool {
	return t == stdContextType || t == netContextType
}

var modVersionPat = regexp.MustCompile("@v[^/]+")

// fileKey finds a stable representation of the caller's file path.
// For calls from package main: strip all leading path entries, leaving just the filename.
// For calls from anywhere else, strip $GOPATH/src, leaving just the package path and file path.
func fileKey(file string) (string, error) {
	if !internal.IsSecondGen() {
		return file, nil
	}
	// If the caller is in the same Dir as mainPath, then strip everything but the file name.
	if filepath.Dir(file) == internal.MainPath {
		return filepath.Base(file), nil
	}
	// If the path contains "gopath/src/", which is what the builder uses for
	// apps which don't use go modules, strip everything up to and including src.
	// Or, if the path starts with /tmp/staging, then we're importing a package
	// from the app's module (and we must be using go modules), and we have a
	// path like /tmp/staging1234/srv/... so strip everything up to and
	// including the first /srv/.
	// And be sure to look at the GOPATH, for local development.
	s := string(filepath.Separator)
	for _, s := range []string{filepath.Join("gopath", "src") + s, s + "srv" + s, filepath.Join(build.Default.GOPATH, "src") + s} {
		if idx := strings.Index(file, s); idx > 0 {
			return file[idx+len(s):], nil
		}
	}

	// Finally, if that all fails then we must be using go modules, and the file is a module,
	// so the path looks like /go/pkg/mod/github.com/foo/bar@v0.0.0-20181026220418-f595d03440dc/baz.go
	// So... remove everything up to and including mod, plus the @.... version string.
	m := "/mod/"
	if idx := strings.Index(file, m); idx > 0 {
		file = file[idx+len(m):]
	} else {
		return file, fmt.Errorf("fileKey: unknown file path format for %q", file)
	}
	return modVersionPat.ReplaceAllString(file, ""), nil
}

// Func declares a new function that can be called in a deferred fashion.
// The second argument i must be a function with the first argument of
// type context.Context.
// To make the key globally unique, the SDK code will combine "key" with
// the filename of the file in which myFunc is defined
// (e.g., /some/path/myfile.go). This is convenient, but can lead to
// failed deferred tasks if you refactor your code, or change from
// GOPATH to go.mod, and then re-deploy with in-flight deferred tasks.
//
// This function Func must be called in a global scope to properly
// register the function with the framework.
//
// Deprecated: Use MustRegister instead.
func Func(key string, i interface{}) *Function {
	// Derive unique, somewhat stable key for this func.
	_, file, _, _ := runtime.Caller(1)
	fk, err := fileKey(file)
	if err != nil {
		// Not fatal, but log the error
		stdlog.Printf("delay: %v", err)
	}
	key = fk + ":" + key
	f, err := registerFunction(key, i)
	if err != nil {
		return f
	}
	if old := funcs[f.key]; old != nil {
		old.err = fmt.Errorf("multiple functions registered for %s", key)
	}
	funcs[f.key] = f
	return f
}

// MustRegister declares a new function that can be called in a deferred fashion.
// The second argument i must be a function with the first argument of
// type context.Context.
// MustRegister requires the key to be globally unique.
//
// This function MustRegister must be called in a global scope to properly
// register the function with the framework.
// See the package notes above for more details.
func MustRegister(key string, i interface{}) *Function {
	f, err := registerFunction(key, i)
	if err != nil {
		panic(err)
	}

	if old := funcs[f.key]; old != nil {
		panic(fmt.Errorf("multiple functions registered for %q", key))
	}
	funcs[f.key] = f
	return f
}

func registerFunction(key string, i interface{}) (*Function, error) {
	f := &Function{fv: reflect.ValueOf(i)}
	f.key = key

	t := f.fv.Type()
	if t.Kind() != reflect.Func {
		f.err = errors.New("not a function")
		return f, f.err
	}
	if t.NumIn() == 0 || !isContext(t.In(0)) {
		f.err = errFirstArg
		return f, errFirstArg
	}

	// Register the function's arguments with the gob package.
	// This is required because they are marshaled inside a []interface{}.
	// gob.Register only expects to be called during initialization;
	// that's fine because this function expects the same.
	for i := 0; i < t.NumIn(); i++ {
		// Only concrete types may be registered. If the argument has
		// interface type, the client is resposible for registering the
		// concrete types it will hold.
		if t.In(i).Kind() == reflect.Interface {
			continue
		}
		gob.Register(reflect.Zero(t.In(i)).Interface())
	}
	return f, nil
}

type invocation struct {
	Key  string
	Args []interface{}
}

// Call invokes a delayed function.
//
//	err := f.Call(c, ...)
//
// is equivalent to
//
//	t, _ := f.Task(...)
//	_, err := taskqueue.Add(c, t, "")
func (f *Function) Call(c context.Context, args ...interface{}) error {
	t, err := f.Task(args...)
	if err != nil {
		return err
	}
	_, err = taskqueueAdder(c, t, queue)
	return err
}

// Task creates a Task that will invoke the function.
// Its parameters may be tweaked before adding it to a queue.
// Users should not modify the Path or Payload fields of the returned Task.
func (f *Function) Task(args ...interface{}) (*taskqueue.Task, error) {
	if f.err != nil {
		return nil, fmt.Errorf("delay: func is invalid: %v", f.err)
	}

	nArgs := len(args) + 1 // +1 for the context.Context
	ft := f.fv.Type()
	minArgs := ft.NumIn()
	if ft.IsVariadic() {
		minArgs--
	}
	if nArgs < minArgs {
		return nil, fmt.Errorf("delay: too few arguments to func: %d < %d", nArgs, minArgs)
	}
	if !ft.IsVariadic() && nArgs > minArgs {
		return nil, fmt.Errorf("delay: too many arguments to func: %d > %d", nArgs, minArgs)
	}

	// Check arg types.
	for i := 1; i < nArgs; i++ {
		at := reflect.TypeOf(args[i-1])
		var dt reflect.Type
		if i < minArgs {
			// not a variadic arg
			dt = ft.In(i)
		} else {
			// a variadic arg
			dt = ft.In(minArgs).Elem()
		}
		// nil arguments won't have a type, so they need special handling.
		if at == nil {
			// nil interface
			switch dt.Kind() {
			case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
				continue // may be nil
			}
			return nil, fmt.Errorf("delay: argument %d has wrong type: %v is not nilable", i, dt)
		}
		switch at.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
			av := reflect.ValueOf(args[i-1])
			if av.IsNil() {
				// nil value in interface; not supported by gob, so we replace it
				// with a nil interface value
				args[i-1] = nil
			}
		}
		if !at.AssignableTo(dt) {
			return nil, fmt.Errorf("delay: argument %d has wrong type: %v is not assignable to %v", i, at, dt)
		}
	}

	inv := invocation{
		Key:  f.key,
		Args: args,
	}

	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(inv); err != nil {
		return nil, fmt.Errorf("delay: gob encoding failed: %v", err)
	}

	return &taskqueue.Task{
		Path:    path,
		Payload: buf.Bytes(),
	}, nil
}

// Request returns the special task-queue HTTP request headers for the current
// task queue handler. Returns an error if called from outside a delay.Func.
func RequestHeaders(c context.Context) (*taskqueue.RequestHeaders, error) {
	if ret, ok := c.Value(headersContextKey).(*taskqueue.RequestHeaders); ok {
		return ret, nil
	}
	return nil, errOutsideDelayFunc
}

var taskqueueAdder = taskqueue.Add // for testing

func init() {
	http.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		runFunc(appengine.NewContext(req), w, req)
	})
}

func runFunc(c context.Context, w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	c = context.WithValue(c, headersContextKey, taskqueue.ParseRequestHeaders(req.Header))

	var inv invocation
	if err := gob.NewDecoder(req.Body).Decode(&inv); err != nil {
		log.Errorf(c, "delay: failed decoding task payload: %v", err)
		log.Warningf(c, "delay: dropping task")
		return
	}

	f := funcs[inv.Key]
	if f == nil {
		log.Errorf(c, "delay: no func with key %q found", inv.Key)
		log.Warningf(c, "delay: dropping task")
		return
	}

	ft := f.fv.Type()
	in := []reflect.Value{reflect.ValueOf(c)}
	for _, arg := range inv.Args {
		var v reflect.Value
		if arg != nil {
			v = reflect.ValueOf(arg)
		} else {
			// Task was passed a nil argument, so we must construct
			// the zero value for the argument here.
			n := len(in) // we're constructing the nth argument
			var at reflect.Type
			if !ft.IsVariadic() || n < ft.NumIn()-1 {
				at = ft.In(n)
			} else {
				at = ft.In(ft.NumIn() - 1).Elem()
			}
			v = reflect.Zero(at)
		}
		in = append(in, v)
	}
	out := f.fv.Call(in)

	if n := ft.NumOut(); n > 0 && ft.Out(n-1) == errorType {
		if errv := out[n-1]; !errv.IsNil() {
			log.Errorf(c, "delay: func failed (will retry): %v", errv.Interface())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
