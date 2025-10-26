# Stage 1: Build
FROM golang:1.23-alpine AS builder

# Install git for fetching dependencies (e.g., from GitHub)
RUN apk add --no-cache git

# Set the working directory inside the container
WORKDIR /app

# Copy module files and download dependencies (better layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go binary with size optimizations
RUN go build -ldflags="-w -s" -o /bin/app

# Stage 2: Minimal runtime container
FROM alpine:latest

# Copy only the compiled binary
COPY --from=builder /bin/app /app

# Set a default port environment variable
ENV PORT=8080
ENV ROOTED_DB=""
EXPOSE 8080

# Run the app
ENTRYPOINT ["/app"]
