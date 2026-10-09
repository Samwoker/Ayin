# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/mcp ./cmd/mcp && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/triage ./cmd/triage && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/eval ./cmd/eval && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/simulate ./cmd/simulate && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/seed ./cmd/seed

# Production runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/api /app/api
COPY --from=builder /bin/worker /app/worker
COPY --from=builder /bin/mcp /app/mcp
COPY --from=builder /bin/triage /app/triage
COPY --from=builder /bin/eval /app/eval
COPY --from=builder /bin/simulate /app/simulate
COPY --from=builder /bin/seed /app/seed

USER nonroot:nonroot

EXPOSE 8080 9090

ENTRYPOINT ["/app/api"]
