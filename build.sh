#!/bin/bash

if [[ -z "${VERSION}" ]]
then
    echo 'the VERSION environment variable must be set'
    exit 1
fi

set -eux

buildDir='build'
mkdir -p "${buildDir}"

projectName="$(basename $( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd ))"
filename="${projectName}"
if [[ ! -z "${GOOS+x}" ]]
then
    filename="${filename}-${GOOS}"
fi
if [[ ! -z "${GOARCH+x}" ]]
then
    filename="${filename}-${GOARCH}"
fi
if [[ ! -z "${GOOS+x}" ]] && [[ "${GOOS}" == "windows" ]]
then
    filename="${filename}.exe"
fi

go build -ldflags "-X main.version=${VERSION}" -o "${buildDir}/${filename}" cmd/${projectName}/main.go
