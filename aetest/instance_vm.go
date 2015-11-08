// +build !appengine

package aetest

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine/internal"
)

// NewInstance launches a running instance of api_server.py which can be used
// for multiple test Contexts that delegate all App Engine API calls to that
// instance.
// If opts is nil the default values are used.
func NewInstance(opts *Options) (Instance, error) {
	i := &instance{
		opts:  opts,
		appID: "testapp",
	}
	if opts != nil && opts.AppID != "" {
		i.appID = opts.AppID
	}
	if err := i.startChild(); err != nil {
		return nil, err
	}
	return i, nil
}

func newSessionID() string {
	var buf [16]byte
	io.ReadFull(rand.Reader, buf[:])
	return fmt.Sprintf("%x", buf[:])
}

// instance implements the Instance interface.
type instance struct {
	opts     *Options
	child    *exec.Cmd
	apiURL   *url.URL // base URL of API HTTP server
	adminURL string   // base URL of admin HTTP server
	appDir   string
	appID    string
	relFuncs []func() // funcs to release any associated contexts
}

// NewRequest returns an *http.Request associated with this instance.
func (i *instance) NewRequest(method, urlStr string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}

	// Associate this request.
	release := internal.RegisterTestRequest(req, i.apiURL, func(ctx context.Context) context.Context {
		ctx = internal.WithAppIDOverride(ctx, "dev~"+i.appID)
		return ctx
	})
	i.relFuncs = append(i.relFuncs, release)

	return req, nil
}

// Close kills the child api_server.py process, releasing its resources.
func (i *instance) Close() (err error) {
	for _, rel := range i.relFuncs {
		rel()
	}
	i.relFuncs = nil
	if i.child == nil {
		return nil
	}
	defer func() {
		i.child = nil
		err1 := os.RemoveAll(i.appDir)
		if err == nil {
			err = err1
		}
	}()

	if p := i.child.Process; p != nil {
		errc := make(chan error, 1)
		go func() {
			errc <- i.child.Wait()
		}()

		// Call the quit handler on the admin server.
		res, err := http.Get(i.adminURL + "/quit")
		if err != nil {
			p.Kill()
			return fmt.Errorf("unable to call /quit handler: %v", err)
		}
		res.Body.Close()

		select {
		case <-time.After(15 * time.Second):
			p.Kill()
			return errors.New("timeout killing child process")
		case err = <-errc:
			// Do nothing.
		}
	}
	return
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func findPython() (path string, err error) {
	for _, name := range []string{"python2.7", "python"} {
		path, err = exec.LookPath(name)
		if err == nil {
			return
		}
	}
	return
}

func findDevAppserver() (string, error) {
	if p := os.Getenv("APPENGINE_DEV_APPSERVER"); p != "" {
		if fileExists(p) {
			return p, nil
		}
		return "", fmt.Errorf("invalid APPENGINE_DEV_APPSERVER environment variable; path %q doesn't exist", p)
	}
	return exec.LookPath("dev_appserver.py")
}

var apiServerAddrRE = regexp.MustCompile(`Starting API server at: (\S+)`)
var adminServerAddrRE = regexp.MustCompile(`Starting admin server at: (\S+)`)

func (i *instance) startChild() (err error) {
	if PrepareDevAppserver != nil {
		if err := PrepareDevAppserver(); err != nil {
			return err
		}
	}
	python, err := findPython()
	if err != nil {
		return fmt.Errorf("Could not find python interpreter: %v", err)
	}
	devAppserver, err := findDevAppserver()
	if err != nil {
		return fmt.Errorf("Could not find dev_appserver.py: %v", err)
	}

	i.appDir, err = ioutil.TempDir("", "appengine-aetest")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(i.appDir)
		}
	}()
	err = os.Mkdir(filepath.Join(i.appDir, "app"), 0755)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(i.appDir, "app", "app.yaml"), []byte(i.appYAML()), 0644)
	if err != nil {
		return err
	}
	if i.opts != nil && len(i.opts.TaskQueues) > 0 {
		err = ioutil.WriteFile(filepath.Join(i.appDir, "app", "queue.yaml"), i.queueYAML(), 0644)
		if err != nil {
			return err
		}
	}
	err = ioutil.WriteFile(filepath.Join(i.appDir, "app", "stubapp.go"), []byte(appSource), 0644)
	if err != nil {
		return err
	}

	appserverArgs := []string{
		devAppserver,
		"--port=0",
		"--api_port=0",
		"--admin_port=0",
		"--automatic_restart=false",
		"--skip_sdk_update_check=true",
		"--clear_datastore=true",
		"--clear_search_indexes=true",
		"--enable_task_running=false",
		"--datastore_path", filepath.Join(i.appDir, "datastore"),
	}
	if i.opts != nil && i.opts.StronglyConsistentDatastore {
		appserverArgs = append(appserverArgs, "--datastore_consistency_policy=consistent")
	}
	appserverArgs = append(appserverArgs, filepath.Join(i.appDir, "app"))

	i.child = exec.Command(python,
		appserverArgs...,
	)
	i.child.Stdout = os.Stdout
	var stderr io.Reader
	stderr, err = i.child.StderrPipe()
	if err != nil {
		return err
	}
	stderr = io.TeeReader(stderr, os.Stderr)
	if err = i.child.Start(); err != nil {
		return err
	}

	// Wait until we have read the URLs of the API server and admin interface.
	errc := make(chan error, 1)
	apic := make(chan *url.URL)
	adminc := make(chan string)
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			if match := apiServerAddrRE.FindStringSubmatch(s.Text()); match != nil {
				u, err := url.Parse(match[1])
				if err != nil {
					errc <- fmt.Errorf("failed to parse API URL %q: %v", match[1], err)
					return
				}
				apic <- u
			}
			if match := adminServerAddrRE.FindSubmatch(s.Bytes()); match != nil {
				adminc <- string(match[1])
			}
		}
		if err = s.Err(); err != nil {
			errc <- err
		}
	}()

	for i.apiURL == nil || i.adminURL == "" {
		select {
		case i.apiURL = <-apic:
		case i.adminURL = <-adminc:
		case <-time.After(15 * time.Second):
			if p := i.child.Process; p != nil {
				p.Kill()
			}
			return errors.New("timeout starting child process")
		case err := <-errc:
			return fmt.Errorf("error reading child process stderr: %v", err)
		}
	}
	return nil
}

func (i *instance) appYAML() string {
	return fmt.Sprintf(appYAMLTemplate, i.appID)
}

const appYAMLTemplate = `
application: %s
version: 1
runtime: go
api_version: go1
vm: true

handlers:
- url: /.*
  script: _go_app
`

const appSource = `
package main
import "google.golang.org/appengine"
func main() { appengine.Main() }
`

func (i *instance) queueYAML() []byte {
	queueBuf := bytes.Buffer{}
	if i.opts != nil && len(i.opts.TaskQueues) > 0 {
		queueTempl.ExecuteTemplate(&queueBuf, "queue.yaml", i.opts.TaskQueues)
	}
	return queueBuf.Bytes()
}

var queueTempl = template.Must(template.New("queue.yaml").Parse(`
total_storage_limit: 120M
queue:{{range .}}
- name: {{.}}
  rate: 35/s{{end}}
`))
