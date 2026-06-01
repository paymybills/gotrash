# --- Stage 1: Build standalone static Go binary ---
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy dependency files and fetch modules
COPY go.mod ./
RUN go mod download

# Copy the rest of the source tree
COPY . .

# Compile as a fully static, standalone binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gotrash main.go

# --- Stage 2: Create a secure, lightweight runtime ---
FROM alpine:3.19

WORKDIR /app

# Install standard CA certificates for outbound HTTP support if needed
RUN apk --no-cache add ca-certificates

# Copy compiled standalone executable
COPY --from=builder /build/gotrash /app/gotrash

# Pre-create data directory for persistent uploads
RUN mkdir -p /app/data/uploads

# Expose port (standard container convention, overridden dynamically by Railway $PORT)
EXPOSE 8080

# Run daemon pointing to persistent directory
ENTRYPOINT ["/app/gotrash", "-dir", "/app/data/uploads"]
