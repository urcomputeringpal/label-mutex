# Copyright 2020 Seth Vargo
# Copyright 2023 Ur Computering Pal, LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Specify the version of Go to use
FROM golang:1.21@sha256:c416ceeec1cdf037b80baef1ccb402c230ab83a9134b34c0902c542eb4539c82 as builder

RUN apt-get update && apt-get install -y xz-utils && rm -rf /var/lib/apt/lists/*

# Install upx (upx.github.io) to compress the compiled action
RUN curl -L https://github.com/upx/upx/releases/download/v4.1.0/upx-4.1.0-amd64_linux.tar.xz | \
    tar -xJv --strip-components=1

# Turn on Go modules support and disable CGO
ENV GO111MODULE=on CGO_ENABLED=0

# Copy all the files from the host into the container
WORKDIR /src
COPY . .

# Compile the action - the added flags instruct Go to produce a
# standalone binary
RUN go build \
  -a \
  -trimpath \
  -ldflags "-s -w -extldflags '-static'" \
  -installsuffix cgo \
  -tags netgo \
  -o /bin/action \
  .

# Strip any symbols - this is not a library
RUN strip /bin/action

# Compress the compiled action
RUN /go/upx -q -9 /bin/action


# Step 2

# Use the most basic and empty container - this container has no
# runtime, files, shell, libraries, etc.
FROM scratch

# Copy over SSL certificates from the first step - this is required
# if our code makes any outbound SSL connections because it contains
# the root CA bundle.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy over the compiled action from the first step
COPY --from=builder /bin/action /bin/action

# Specify the container's entrypoint as the action
ENTRYPOINT ["/bin/action"]
