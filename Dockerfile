# syntax=docker/dockerfile:1

# 1. Frontend: build the SPA into web/dist. Its output is platform-independent, so it
#    always runs natively on the build machine.
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY api/ /src/api/
COPY design/ /src/design/
COPY assets/ /src/assets/
COPY web/ ./
RUN npm run build

# 2. Server: one static, CGO-free binary with the SPA embedded, cross-compiled natively
#    for the target platform (no emulation needed).
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=dev
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
      -ldflags "-s -w -X github.com/reilandeubank/grower/internal/buildinfo.Version=${VERSION}" \
      -o /out/grower ./cmd/grower

# 3. Runtime: distroless, no shell. Health is checked by the binary itself.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/grower /usr/local/bin/grower
ENV GROWER_DATA_DIR=/data \
    GROWER_LISTEN=:8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/usr/local/bin/grower", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/grower"]
CMD ["serve"]
