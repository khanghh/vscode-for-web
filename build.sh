#!/bin/bash

# Build script for VS Code Web

set -e

# Define VS Code version
vscodeVersion="1.105.1"

# Clone vscode if not exists
if [ ! -d "vscode" ]; then
    git clone --depth 1 https://github.com/microsoft/vscode.git --branch $vscodeVersion vscode 
fi

# Build vscode web version
echo "Installing dependencies and building vscode web version..."
cd vscode && npm install && npm run gulp vscode-web-min
echo "Build completed"

# Build the custom extension
# echo "Building remote source extension..."
# cd ../remote-source-extension
# npm install
# npm run compile
# cd ../vscode

