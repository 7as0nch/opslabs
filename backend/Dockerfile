FROM golang:1.25-alpine AS builder

WORKDIR /src

# Pre-download dependencies to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/backend ./cmd/backend

FROM alpine:latest

# Install basic dependencies and set timezone
RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# Copy binary
COPY --from=builder /app/backend .

EXPOSE 8000
EXPOSE 9000

VOLUME ["/data/conf", "/app/logs"]

ENTRYPOINT ["./backend"]
CMD ["-conf", "/data/conf/config.yaml"]
