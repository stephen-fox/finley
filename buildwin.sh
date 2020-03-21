#!/bin/bash

set -eux

projectDirPath="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

GOOS=windows "${projectDirPath}/build.sh"
