# Go App Engine for Managed VMs

This repository supports the Go runtime for Managed VMs on App Engine.
It provides APIs for interacting with App Engine services.
Its canonical import path is `google.golang.org/appengine`.

## Directory structure
The top level directory of this repository is the `appengine` package. It
contains the
basic types (e.g. `appengine.Context`) that are used across APIs. Specific API
packages are in subdirectories (e.g. `datastore`).

There is an `internal` subdirectory that contains service protocol buffers,
plus packages required for connectivity to make API calls. App Engine apps
should not directly import any package under `internal`.
