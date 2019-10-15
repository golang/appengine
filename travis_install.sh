#!/bin/bash
set -e

if [[ $GO111MODULE == "on" ]]; then
  go get .
else
  go get -u -v $(go list -f '{{join .Imports "\n"}}{{"\n"}}{{join .TestImports "\n"}}' ./... | sort | uniq | grep -v appengine)
fi
