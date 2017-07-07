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

FROM docker:stable-dind

RUN apk add --update \
    iptables \
    ethtool \
    ca-certificates \
    util-linux \
    socat \
    conntrack-tools \
    && rm -rf /var/cache/apk/*

# Copy over important files
COPY out/localkube /
COPY deploy/addons/addon-manager.yaml /etc/kubernetes/manifests/addon-manager.yaml
COPY deploy/addons /etc/kubernetes/addons
CMD  /localkube start \
    --apiserver-insecure-address=127.0.0.1 \
    --apiserver-insecure-port=8080 \
    --logtostderr=true