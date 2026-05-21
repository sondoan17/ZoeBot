# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata

# Copy source code first
COPY . .

# Download dependencies and build
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o zoebot ./cmd/zoebot

# Runtime image with tools required for YouTube audio playback
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata ffmpeg yt-dlp

# Copy binary
COPY --from=builder /app/zoebot /zoebot

# Copy data files
COPY --from=builder /app/data /data

# Set timezone
ENV TZ=Asia/Ho_Chi_Minh
ENV MUSIC_TMP_DIR=/tmp

# Expose health check port
EXPOSE 8080

# Run the bot
ENTRYPOINT ["/zoebot"]
