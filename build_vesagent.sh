#!/bin/bash
#
#  Copyright (c) 2019 AT&T Intellectual Property.
#  Copyright (c) 2018-2019 Nokia.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
#

set -e
set -x
# Load modules
GO111MODULE=on go mod download
export GOPATH=$HOME/go
export PATH=$GOPATH/bin:$GOROOT/bin:$PATH

# Run vesmgr UT
go test ./...

# setup version tag
if [ -f container-tag.yaml ]
then
    tag=$(grep "tag:" container-tag.yaml | awk '{print $2}')
else
    tag="-"
fi

hash=$(git rev-parse --short HEAD || true)

# Install vesagent
go install -ldflags "-X main.Version=$tag -X main.Hash=$hash" -v ./cmd/vesmgr


