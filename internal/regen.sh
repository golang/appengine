#!/bin/bash -e

PKG=github.com/golang/appengine

function die() {
	echo 1>&2 $*
	exit 1
}

# Sanity check that the right tools are accessible.
for tool in go protoc protoc-gen-go; do
	q=$(which $tool) || die "didn't find $tool"
	echo 1>&2 "$tool: $q"
done

echo -n 1>&2 "finding package dir... "
pkgdir=$(go list -f '{{.Dir}}' $PKG)
echo 1>&2 $pkgdir
base=$(echo $pkgdir | sed "s,/$PKG\$,,")
echo 1>&2 "base: $base"
cd $base
for f in $(find $PKG/internal -name '*.proto'); do
	echo 1>&2 "* $f"
	protoc --go_out=. $f
done
