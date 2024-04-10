#!/bin/bash -e
#
# This script rebuilds the generated code for the protocol buffers.
# To run this you will need protoc and goprotobuf installed;
# see https://google.golang.org/protobuf for instructions.

PKG=google.golang.org/appengine

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

# Run protoc once per package.
for dir in $(find $PKG/internal -name '*.proto' | xargs dirname | sort | uniq); do
	echo 1>&2 "* $dir"
	bname=$(basename $dir)
	pushd ${pkgdir}/internal/${bname}
	echo "cmd:" protoc -I=.  -I=${base}  --go_opt=paths=source_relative --go_out=. *.proto
	protoc -I=. -I=${base} --go_opt=paths=source_relative --go_out=. *.proto
	popd
done
