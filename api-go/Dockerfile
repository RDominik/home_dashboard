## Build stage
FROM golang:1.24-alpine AS builder

# git + Docker CLI für System-Update
RUN apk add --no-cache git

WORKDIR /build
COPY api-go/go.mod api-go/go.sum* ./
RUN go mod download
COPY api-go/ .
COPY api-go/mqtt/broker_config.json ./mqtt/broker_config.json
RUN go mod tidy && CGO_ENABLED=0 go build -o /app/server .

## Runtime stage
FROM alpine:3.19

# git + Docker CLI für System-Update
RUN apk add --no-cache git docker-cli docker-cli-compose ca-certificates && \
    git config --global --add safe.directory /repo

WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /build/mqtt/broker_config.json ./mqtt/broker_config.json

EXPOSE 8083
CMD ["./server"]
