# Build Code Web
FROM mcr.microsoft.com/devcontainers/typescript-node:16-bullseye AS web-builder

RUN apt-get update && apt-get install -y libsecret-1-dev libxkbfile-dev
RUN git config --system codespaces-theme.hide-status 1
RUN yarn global add node-gyp@9.3.1

USER node
WORKDIR /workdir

COPY ./web .

RUN ./build.sh --all

# Build golang server
FROM golang:1.25.1-alpine AS go-builder

RUN apk add --no-cache git ca-certificates tzdata && update-ca-certificates

WORKDIR /workdir
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "\
      -w -s \
      -X 'main.gitCommit=$(git rev-parse HEAD)' \
      -X 'main.gitDate=$(git show -s --format=%cI HEAD)' \
      -X 'main.gitTag=$(git describe --tags --always --dirty)'" \
    -o server .

# Final image
FROM scratch

WORKDIR /app

COPY --from=go-builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=go-builder /workdir/server /app/server
COPY --from=web-builder /workdir/dist /app/dist

ENTRYPOINT ["/app/server"]
