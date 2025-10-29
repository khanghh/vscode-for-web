FROM mcr.microsoft.com/devcontainers/typescript-node:16-bullseye

RUN apt-get update && apt-get install -y libsecret-1-dev libxkbfile-dev
RUN git config --system codespaces-theme.hide-status 1
RUN yarn global add node-gyp@9.3.1

USER node
WORKDIR /workdir

EXPOSE 3000 8080

CMD ["sh", "-c", "trap 'exit 0' INT TERM; while true; do sleep 1; done"]
