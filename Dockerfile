FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /homelab-dash ./cmd/server

FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /homelab-dash /usr/local/bin/homelab-dash

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/homelab-dash"]
CMD ["--config", "/config/homelab.yaml", "--db", "/data/homelab.db"]
