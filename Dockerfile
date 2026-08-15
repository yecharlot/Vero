# Build
FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o vero ./cmd/vero

# Run
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/vero .
ENV PORT=8080
ENV VERO_DATA_DIR=/var/data/vero
EXPOSE 8080
CMD ["./vero"]
