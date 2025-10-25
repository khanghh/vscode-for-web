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
cp -r ./vscode-web ./dist && cp index.html ./dist/index.html

echo "Build completed"
