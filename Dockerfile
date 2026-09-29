# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/...

# distroless/static has CA certificates and runs as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot
# One JSON object per line, for Kubernetes log collectors.
ENV LOG_FORMAT=json
ENV GIN_MODE=release
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
