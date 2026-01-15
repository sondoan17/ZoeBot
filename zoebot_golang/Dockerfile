# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build with maximum optimizations for size and performance
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static'" \
    -trimpath \
    -o zoebot ./cmd/zoebot

# Use scratch (empty) image for minimal size
FROM scratch

# Copy certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /app/zoebot /zoebot

# Copy data files
COPY --from=builder /app/data /data

# Set timezone
ENV TZ=Asia/Ho_Chi_Minh

# Expose health check port
EXPOSE 8080

# Run the bot
ENTRYPOINT ["/zoebot"]
