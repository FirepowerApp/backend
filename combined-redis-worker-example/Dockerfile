FROM golang:1.23.3 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/worker

# Final stage - includes both Redis and worker
FROM redis:7-alpine

# Install any additional tools if needed
RUN apk add --no-cache ca-certificates

# Copy worker binary from builder
COPY --from=builder /app/worker /usr/local/bin/worker

# Copy startup script
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Expose Redis port
EXPOSE 6379

# Use custom entrypoint that starts both
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
