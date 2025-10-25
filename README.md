# VS Code Web Builder

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

This will clone the VS Code source code (if not present) and build the web version using `gulp vscode-web-min`.

## Extension

The `remote-source-extension` provides a command to load source code from remote URLs.

- Command: `Remote Source Loader: Load Remote Source`
- Prompts for a URL, fetches the content, and opens it in a new editor tab with appropriate syntax highlighting.

## Development

- VS Code source is cloned into `vscode-source/`
- Built web version will be in `vscode-source/vscode-web/`
- Extension source is in `remote-source-extension/`

## Docker

- Uses `node:22-bullseye` base image
- Includes tools like tmux, vim, python, etc.
- Mounts the project directory to `/workdir`