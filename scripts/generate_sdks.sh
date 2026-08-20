#!/bin/bash
set -e

# This script generates SDKs for Node, Go, and Python from the OpenAPI spec.

# Ensure we are in the root directory
cd "$(dirname "$0")/.."

OPENAPI_FILE="openapi.yaml"

if [ ! -f "$OPENAPI_FILE" ]; then
    echo "OpenAPI spec file not found: $OPENAPI_FILE"
    exit 1
fi

echo "Generating SDKs..."

# --- Node.js ---
echo "Generating Node.js SDK..."
rm -rf ../sapliy-sdk-node/src/generated
mkdir -p ../sapliy-sdk-node/src/generated
openapi-generator-cli generate -i "$OPENAPI_FILE" -g typescript-axios -o ../sapliy-sdk-node/src/generated --additional-properties=npmName=@sapliyio/sapliy-node-generated,supportsES6=true

# --- Go ---
echo "Generating Go SDK..."
rm -rf ../sapliy-sdk-go/generated
mkdir -p ../sapliy-sdk-go/generated
openapi-generator-cli generate -i "$OPENAPI_FILE" -g go -o ../sapliy-sdk-go/generated \
  --additional-properties=packageName=generated,enumClassPrefix=true,withGoMod=false \
  --git-host github.com --git-user-id sapliy --git-repo-id sapliy-sdk-go

# Fix GIT_USER_ID/GIT_REPO_ID placeholders in generated tests
sed -i '' 's/github.com\/GIT_USER_ID\/GIT_REPO_ID/github.com\/sapliy\/sapliy-sdk-go\/generated/g' ../sapliy-sdk-go/generated/test/*.go

# Fix test imports: generated tests import the root package alias by default; correct to /generated sub-package
sed -i '' 's|openapiclient "github.com/sapliy/sapliy-sdk-go"|openapiclient "github.com/sapliy/sapliy-sdk-go/generated"|g' ../sapliy-sdk-go/generated/test/*.go

# --- Python ---
echo "Generating Python SDK..."
rm -rf ../sapliy-sdk-python/sapliyio_fintech/generated
mkdir -p ../sapliy-sdk-python/sapliyio_fintech/generated
openapi-generator-cli generate -i "$OPENAPI_FILE" -g python -o ../sapliy-sdk-python/sapliyio_fintech/generated \
  --additional-properties=packageName=sapliyio_fintech.generated

# Clean up redundant project files from generated sub-dirs (conflicts with parent wrappers)
rm -f ../sapliy-sdk-go/generated/go.mod ../sapliy-sdk-go/generated/go.sum 2>/dev/null || true
rm -f ../sapliy-sdk-python/sapliyio_fintech/generated/pyproject.toml \
      ../sapliy-sdk-python/sapliyio_fintech/generated/setup.py \
      ../sapliy-sdk-python/sapliyio_fintech/generated/setup.cfg \
      ../sapliy-sdk-python/sapliyio_fintech/generated/requirements.txt \
      ../sapliy-sdk-python/sapliyio_fintech/generated/README.md 2>/dev/null || true

echo "SDK generation complete."
