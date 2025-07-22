# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod./
COPY go.sum./
RUN go mod download
COPY..

# Final stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/app.
# The ENTRYPOINT will be provided in docker-compose.yml
ENTRYPOINT ["./app"]