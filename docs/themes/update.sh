#!/bin/sh -e

cd "$(dirname "$0")"

usage () {
    echo "usage: $0 versionNumber" >&2
    echo "    get versionNumber from:"
    echo "    https://github.com/thegeeklab/hugo-geekdoc/releases"
}

if [ "$#" -ne 1 ]; then
    usage && exit 1
elif [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    usage && exit 0
fi

version="$1"

rm -rf hugo-geekdocs
mkdir hugo-geekdocs
curl -SsL "https://github.com/thegeeklab/hugo-geekdoc/releases/download/$version/hugo-geekdoc.tar.gz" \
    | tar -xzC hugo-geekdocs
