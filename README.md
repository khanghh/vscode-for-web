# 🚀 vscode-for-web

[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org/)
[![Node.js Version](https://img.shields.io/badge/Node.js-16+-green.svg)](https://nodejs.org/)

A minimal VS Code for Web bundle with a custom RemoteFS extension, powered by a lightweight Go backend. Run VS Code in your browser with remote file access on your browser!

## ⚡ Quick Start

1. **Build vscode web and server**  
   ```bash
   make all
   ```

2. **Run the server**  
   ```bash
   cd ./build/bin
   ./server --rootdir $HOME/projects --listen :3000
   ```  
   Using docker
   ```bash
   docker build -t code-server.
   docker run -p 3000:3000 -v $HOME/projects:/projects code-server --rootdir=/projects
   ```

3. **Open in browser**  
   Visit [http://localhost:3000](http://localhost:3000) to access vscode on your browser
    <img src="https://github.com/khanghh/vscode-for-web/blob/screenshots/screenshot.png?raw=true" width="100%"> 
## 📋 Prerequisites

- **Go** 1.25+ (recommended 1.23+)
- **Node.js** 16+ and **Yarn**
- **Git** and **Bash**

*Optional:*  
- `make` for build shortcuts  
- Docker for containerized deployment

## 📜 Makefile Shortcuts

Speed up your workflow with these targets:

- `make server` → Build Go binary to `build/bin/server`
- `make vscode` → Build VS Code web to `build/bin/dist`
- `make extensions` → Build extensions to `build/bin/dist/extensions`
- `make all` → Build everything at once

## 🤝 Contributing

Pull requests are welcome! For major changes, please open an issue first to discuss what you’d like to change.

### Bug Reports & Feature Requests

Please use the issue tracker to report bugs or request new features.

## 📄 License

This project is licensed under the MIT License.
