FROM golang:1.22-alpine AS builder

ARG VERSION=dev
ARG BUILD_TIME=unknown

RUN apk add --no-cache gcc musl-dev git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o /homelab-dash ./cmd/server

FROM alpine:latest

RUN apk add --no-cache ca-certificates smartmontools wget && \
    adduser -D -H homelab

COPY --from=builder /homelab-dash /usr/local/bin/homelab-dash

USER homelab

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/homelab-dash"]
CMD ["--config", "/config/homelab.yaml", "--db", "/data/homelab.db"]
