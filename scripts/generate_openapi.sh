#!/bin/bash
set -e

# This script generates OpenAPI v2 specs from Protocol Buffers using a Docker container.
# It mimics what happens in the GitHub Action.

# Ensure we are in the root directory
cd "$(dirname "$0")/.."

echo "Generating OpenAPI specs..."

# Ensure GOPATH/bin is in PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Check for protoc
if ! command -v protoc &> /dev/null; then
    echo "protoc could not be found. Please install it."
    exit 1
fi

# Check for protoc-gen-openapiv2
if ! command -v protoc-gen-openapiv2 &> /dev/null; then
    echo "protoc-gen-openapiv2 could not be found. Installing..."
    GO111MODULE=on GOFLAGS="" go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
fi

# Run generation
mkdir -p generated/openapi
protoc -I . -I third_party \
  --openapiv2_out=logtostderr=true,allow_merge=true,merge_file_name=sapliy:generated/openapi \
  $(find proto -name "*.proto")

echo "OpenAPI specs generated in generated/openapi/sapliy.swagger.json"
