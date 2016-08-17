#!/bin/bash

# Copyright 2016 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

REPO_PATH="k8s.io/minikube"

# Check for python on host, and use it if possible, otherwise fall back on python dockerized
if [[ -f $(which python 2>&1) ]]; then
	PYTHON="python"
else
	PYTHON="docker run --rm -it -v $(pwd):/minikube -w /minikube python python"
fi


# Run "go test" on packages that have test files.  Also create coverage profile
echo "Running go tests..."
cd ${GOPATH}/src/${REPO_PATH}
rm -f out/coverage.txt
echo "mode: count" > out/coverage.txt
for pkg in $(go list -f '{{ if .TestGoFiles }} {{.ImportPath}} {{end}}' ./...); do
    go test -v $pkg -covermode=count -coverprofile=out/coverage_tmp.txt || ERROR="Error testing $pkg"
    tail -n +2 out/coverage_tmp.txt >> out/coverage.txt || die "Unable to append coverage for $pkg"
done
rm out/coverage_tmp.txt

# Ignore these paths in the following tests.
ignore="vendor\|\_gopath\|assets.go"

# Check gofmt
echo "Checking gofmt..."
diff -u <(echo -n) <(gofmt -l -s . | grep -v ${ignore})

# Check boilerplate
echo "Checking boilerplate..."
BOILERPLATEDIR=./hack/boilerplate
# Grep returns a non-zero exit code if we don't match anything, which is good in this case.
set +e
files=$(${PYTHON} ${BOILERPLATEDIR}/boilerplate.py --rootdir . --boilerplate-dir ${BOILERPLATEDIR} | grep -v $ignore)
set -e
if [ ! -z ${files} ]; then
	echo "Boilerplate missing in: ${files}."
	exit 1
fi

# Check that cobra docs are up to date
# This is done by generating new docs and then seeing if they are different than the committed docs
echo "Checking help documentation..."
if [[ $(git diff) ]]; then
  echo "Skipping help text check because the git state is dirty."
else
  make gendocs
  files=$(git diff)
  if [[ $files ]]; then
    echo "Help text is out of date: $files \n Please run \"make gendocs\"\n and make sure that those doc changes are committed"
    exit 1
  else
    echo "Help text is up to date"
  fi
fi

echo "Checking releases.json schema"
go run deploy/minikube/schema_check.go
