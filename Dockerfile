# syntax=docker/dockerfile:1

# 1. Frontend: build the SPA into web/dist.
FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY api/ /src/api/
COPY design/ /src/design/
COPY assets/ /src/assets/
COPY web/ ./
RUN npm run build

# 2. Server: one static, CGO-free binary with the SPA embedded.
FROM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
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
