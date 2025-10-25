# VSCode For Web Builder

This project builds the web version of VS Code with a custom extension for loading remote source code.

## Setup

1. Start the development container:
   ```bash
   docker-compose -f docker-compose.dev.yml up -d
   ```

2. Enter the container:
   ```bash
   docker exec -it vscode-dev bash
   ```

3. Build the project:
   ```bash
   npm run build
   ```

This will clone the Visual Studo Code source code (if not present) and build the web version using `gulp vscode-web-min`.

## Extension (planning)

- The `remotefs` extension (planned) will provide files and folders from a user-specified root directory on the server running VS Code Web to the workbench's file tree.

## Development

- VS Code source is cloned into `vscode/`
- Built web version will be in `dist`
- Extension source is in `extensions/`

## Docker

- Uses `mcr.microsoft.com/devcontainers/typescript-node:16-bullseye` base image
- Mounts the project directory to `/workdir`
