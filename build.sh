#!/bin/bash

# Build script for VS Code Web

set -euo pipefail

vscodeVersion="1.75.0"

# Clone vscode if not exists
if [ ! -d "vscode" ]; then
    git clone --depth 1 https://github.com/microsoft/vscode.git --branch $vscodeVersion vscode
fi

# Build vscode web version
echo "Installing dependencies and building vscode web version..."
(cd vscode && yarn install --frozen-lockfile && yarn gulp vscode-web-min)

# Copy built files to dist
echo "Creating dist directory..."
rm -rf ./dist
cp -r ./vscode-web/ ./dist && cp index.html ./dist/index.html


# Build remotefs extension
echo "Building remotefs extension..."
(cd extensions/remotefs && yarn install --frozen-lockfile && yarn package-web)
cp -r extensions/remotefs/dist ./dist/extensions/remotefs
cp extensions/remotefs/package.json ./dist/extensions/remotefs/package.json
cp extensions/remotefs/package.nls.json ./dist/extensions/remotefs/package.nls.json

echo "Build completed"
