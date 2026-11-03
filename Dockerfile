# One image: the Go server serves both the API and the built web UI.
# Build stages run on the build machine and cross-compile, so amd64 and arm64 (e.g. Raspberry Pi) build fast.
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN export CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
 && go build -trimpath -ldflags="-s -w" -o /out/bin/memento-redact ./cmd/server \
 && go build -trimpath -ldflags="-s -w" -o /out/bin/memento-redact-cli ./cmd/cli \
 && mkdir -p /out/data

FROM alpine:3.23
WORKDIR /app
COPY config/default-rulesets.yml /app/config/default-rulesets.yml
COPY --from=web /web/dist /app/web
COPY --from=build /out/bin/ /usr/local/bin/
# /app/data holds the editable rulesets; owned by the unprivileged runtime user so a fresh volume is writable.
COPY --from=build --chown=65534:65534 /out/data /app/data
USER 65534:65534
ENV LISTEN_ADDR=:8080 \
    RULES_PATH=/app/data/rulesets.yml \
    DEFAULT_RULES_PATH=/app/config/default-rulesets.yml \
    STATIC_DIR=/app/web
VOLUME /app/data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/health || exit 1
ENTRYPOINT ["memento-redact"]
