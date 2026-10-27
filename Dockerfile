# syntax=docker/dockerfile:1

# ---- 1. Web UI（Next.js の静的エクスポート）----
FROM node:22-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# ---- 2. Go バイナリ（web/out を埋め込む）----
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/out ./web/out
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ripplegraph-server ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ripplegraph ./cmd/ripplegraph

# ---- 3. CA 証明書（社内 CA を追加する）----
# certs/ に置いた *.crt（PEM）を信頼済みの CA に追加する。ステップ3で社内 GitLab に接続するときに使う。
FROM debian:bookworm-slim AS certs
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY certs/ /usr/local/share/ca-certificates/extra/
RUN update-ca-certificates

# ---- 4. 実行イメージ ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/ripplegraph-server /out/ripplegraph /usr/local/bin/
EXPOSE 8080
# 対象リポジトリを /repo にマウントする: docker run -v /path/to/infra:/repo:ro -p 8080:8080 ripplegraph
ENTRYPOINT ["/usr/local/bin/ripplegraph-server", "-repo", "/repo", "-addr", ":8080"]
