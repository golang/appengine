// Copyright 2015 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Program aedeploy assists with deploying App Engine "flexible environment" Go apps to production.
// A temporary directory is created; the app, its subdirectories, and all its
// dependencies from $GOPATH are copied into the directory; then the app
// is deployed to production with the provided command.
//
// The app must be in "package main".
//
// This command must be issued from within the root directory of the app
// (where the app.yaml file is located).
package main

import (
	"flag"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// vendored corresponds to srcDir depending on a vendored version of dir.
// I.e. `import "dir"` from inside srcDir resolves to `.../some/ancestor/vendor/dir`.
type vendored struct {
	srcDir string
	dir    string
}

var (
	skipFiles = map[string]bool{
		".git":        true,
		".gitconfig":  true,
		".hg":         true,
		".travis.yml": true,
	}

	gopathCache   = map[string]string{}
	vendoredCache = map[vendored]string{}
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\t%s gcloud --verbosity debug preview app deploy --version myversion ./app.yaml\tDeploy app to production\n", os.Args[0])
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() < 1 {
		usage()
		os.Exit(1)
	}

	if err := aedeploy(); err != nil {
		fmt.Fprintf(os.Stderr, os.Args[0]+": Error: %v\n", err)
		os.Exit(1)
	}
}

func aedeploy() error {
	tags := []string{"appenginevm"}
	app, err := analyze(tags)
	if err != nil {
		return err
	}

	tmpDir, err := app.bundle()
	if tmpDir != "" {
		defer os.RemoveAll(tmpDir)
	}
	if err != nil {
		return err
	}

	if err := os.Chdir(tmpDir); err != nil {
		return fmt.Errorf("unable to chdir to %v: %v", tmpDir, err)
	}
	return deploy()
}

// deploy calls the provided command to deploy the app from the temporary directory.
func deploy() error {
	cmd := exec.Command(flag.Arg(0), flag.Args()[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unable to run %q: %v", strings.Join(flag.Args(), " "), err)
	}
	return nil
}

type app struct {
	appFiles []string
	imports  map[string]string
}

// analyze checks the app for building with the given build tags and returns
// app files, and a map of full directory import names to original import names.
func analyze(tags []string) (*app, error) {
	ctxt := buildContext(tags)
	appFiles, err := appFiles(ctxt)
	if err != nil {
		return nil, err
	}
	gopath := filepath.SplitList(ctxt.GOPATH)
	im, err := imports(ctxt, ".", gopath)
	return &app{
		appFiles: appFiles,
		imports:  im,
	}, err
}

// buildContext returns the context for building the source.
func buildContext(tags []string) *build.Context {
	return &build.Context{
		GOARCH:    "amd64",
		GOOS:      "linux",
		GOROOT:    build.Default.GOROOT,
		GOPATH:    build.Default.GOPATH,
		Compiler:  build.Default.Compiler,
		BuildTags: append(build.Default.BuildTags, tags...),
	}
}

// bundle bundles the app into a temporary directory.
func (s *app) bundle() (tmpdir string, err error) {
	workDir, err := ioutil.TempDir("", "aedeploy")
	if err != nil {
		return "", fmt.Errorf("unable to create tmpdir: %v", err)
	}

	for srcDir, importName := range s.imports {
		dstDir := "_gopath/src/" + importName
		if err := copyTree(workDir, dstDir, srcDir); err != nil {
			return workDir, fmt.Errorf("unable to copy directory %v to %v: %v", srcDir, dstDir, err)
		}
	}
	if err := copyTree(workDir, ".", "."); err != nil {
		return workDir, fmt.Errorf("unable to copy root directory to /app: %v", err)
	}
	return workDir, nil
}

// imports returns a map of all import directories (recursively) used by the app.
// The return value maps full directory names to original import names.
func imports(ctxt *build.Context, srcDir string, gopath []string) (map[string]string, error) {
	pkg, err := ctxt.ImportDir(srcDir, 0)
	if err != nil {
		return nil, err
	}

	// Resolve imports, preferring vendored packages, then packages in the GOPATH.
	// Any package that could not be resolved and does not contain a "."
	// is assumed to be part of the standard libarry and therefore ignored.
	// Otherwise, unresolved packages will return an error.
	result := make(map[string]string)
	for _, v := range pkg.Imports {
		src, verr := findVendored(srcDir, v, gopath)
		if verr != nil {
			var perr error
			src, perr = findInGopath(v, gopath)
			if perr != nil {
				if !strings.Contains(v, ".") {
					continue
				}
				return nil, fmt.Errorf("unable to find import %v: %v, %v", v, perr, verr)
			}
		}

		if _, ok := result[src]; ok { // Already processed
			continue
		}
		result[src] = v
		im, err := imports(ctxt, src, gopath)
		if err != nil {
			return nil, fmt.Errorf("unable to parse package %v: %v", src, err)
		}
		for k, v := range im {
			result[k] = v
		}
	}
	return result, nil
}

// findVendored searches up the tree for vendor directories containing the named import directory.
func findVendored(srcDir, dir string, gopath []string) (string, error) {
	if os.Getenv("GO15VENDOREXPERIMENT") != "0" {
		srcDir, err := filepath.Abs(srcDir)
		if err != nil {
			return "", fmt.Errorf("unable to search vendor directories: %v", err)
		}

		if v, ok := vendoredCache[vendored{srcDir, dir}]; ok {
			return v, nil
		}

		// srcDirs collects the directories we see as we walk up the tree.
		// All of these directories, if they depend on a vendored version of dir,
		// will depend on the same one.
		var srcDirs []string

		// Walk up the directory tree, looking for the vendored dir.
		for s := srcDir; ; s = filepath.Dir(s) {
			// Don't look in vendor directories outside of the GOPATH.
			var inGopath bool
			for _, p := range gopath {
				if strings.HasPrefix(s, p) {
					inGopath = true
					break
				}
			}
			if !inGopath {
				break
			}

			srcDirs = append(srcDirs, s)
			dst := filepath.Join(s, "vendor", dir)
			if _, err := os.Stat(dst); err == nil {
				for _, sd := range srcDirs {
					vendoredCache[vendored{sd, dir}] = dst
				}
				return dst, nil
			}

			// We got to the root directory, but haven't found the vendored dir.
			// This check isn't used as the loop conditional
			// because we want the loop to run at least once.
			if s == filepath.Dir(s) {
				break
			}
		}
		return "", fmt.Errorf("unable to find package %v in vendor directories at or above %v", dir, srcDir)
	}
	return "", fmt.Errorf("vendoring is disabled")
}

// findInGopath searches the gopath for the named import directory.
func findInGopath(dir string, gopath []string) (string, error) {
	if v, ok := gopathCache[dir]; ok {
		return v, nil
	}
	for _, v := range gopath {
		dst := filepath.Join(v, "src", dir)
		if _, err := os.Stat(dst); err == nil {
			gopathCache[dir] = dst
			return dst, nil
		}
	}
	return "", fmt.Errorf("unable to find package %v in gopath %v", dir, gopath)
}

// copyTree copies srcDir to dstDir relative to dstRoot, ignoring skipFiles.
func copyTree(dstRoot, dstDir, srcDir string) error {
	d := filepath.Join(dstRoot, dstDir)
	if err := os.MkdirAll(d, 0755); err != nil {
		return fmt.Errorf("unable to create directory %q: %v", d, err)
	}

	entries, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("unable to read dir %q: %v", srcDir, err)
	}
	for _, entry := range entries {
		n := entry.Name()
		if skipFiles[n] {
			continue
		}
		s := filepath.Join(srcDir, n)
		if entry.Mode()&os.ModeSymlink == os.ModeSymlink {
			if entry, err = os.Stat(s); err != nil {
				return fmt.Errorf("unable to stat %v: %v", s, err)
			}
		}
		d := filepath.Join(dstDir, n)
		if entry.IsDir() {
			if err := copyTree(dstRoot, d, s); err != nil {
				return fmt.Errorf("unable to copy dir %q to %q: %v", s, d, err)
			}
			continue
		}
		if err := copyFile(dstRoot, d, s); err != nil {
			return fmt.Errorf("unable to copy dir %q to %q: %v", s, d, err)
		}
	}
	return nil
}

// copyFile copies src to dst relative to dstRoot.
func copyFile(dstRoot, dst, src string) error {
	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("unable to open %q: %v", src, err)
	}
	defer s.Close()

	dst = filepath.Join(dstRoot, dst)
	d, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("unable to create %q: %v", dst, err)
	}
	_, err = io.Copy(d, s)
	if err != nil {
		d.Close() // ignore error, copy already failed.
		return fmt.Errorf("unable to copy %q to %q: %v", src, dst, err)
	}
	if err := d.Close(); err != nil {
		return fmt.Errorf("unable to close %q: %v", dst, err)
	}
	return nil
}

// appFiles returns a list of all Go source files in the app.
func appFiles(ctxt *build.Context) ([]string, error) {
	pkg, err := ctxt.ImportDir(".", 0)
	if err != nil {
		return nil, err
	}
	if !pkg.IsCommand() {
		return nil, fmt.Errorf(`the root of your app needs to be package "main" (currently %q). Please see https://cloud.google.com/appengine/docs/flexible/go/ for more details on structuring your app.`, pkg.Name)
	}
	var appFiles []string
	for _, f := range pkg.GoFiles {
		n := filepath.Join(".", f)
		appFiles = append(appFiles, n)
	}
	return appFiles, nil
}
