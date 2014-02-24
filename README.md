# Go App Engine for Managed VMs

This repository supports the Go runtime for Managed VMs on App Engine.
It provides APIs for interacting with App Engine services.
Its canonical import path is `google.golang.org/appengine`.

See https://groups.google.com/d/topic/google-appengine/gRZNqlQPKys/discussion
for more information.

## Directory structure
The top level directory of this repository is the `appengine` package. It
contains the
basic types (e.g. `appengine.Context`) that are used across APIs. Specific API
packages are in subdirectories (e.g. `datastore`).

There is an `internal` subdirectory that contains service protocol buffers,
plus packages required for connectivity to make API calls. App Engine apps
should not directly import any package under `internal`.

## Updating a Go App Engine app

This section describes how to update a traditional Go App Engine app to run on Managed VMs.

### 1. Update YAML files

The `app.yaml` file (and YAML files for modules) should have these new lines added:
```
vm: true
manual_scaling:
  instances: 1
```
See [https://developers.google.com/appengine/docs/go/modules/#Go_Instance_scaling_and_class] for details.

### 2. Update import paths

The import paths for App Engine API packages need to be made relative to `google.golang.org/appengine`.
You can do that manually, or by running this command to recursively update all Go source files in the current directory:
(may require GNU sed)
```
sed -i '/"appengine/{s,"appengine,"google.golang.org/appengine,;s,appengine_,appengine/,}' \
  $(find . -name '*.go')
```
