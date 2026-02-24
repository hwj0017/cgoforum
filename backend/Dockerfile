# Build stage
FROM golang:1.26.1-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN set -eux; \
    ok=0; \
    for i in 1 2 3 4 5; do \
    if go mod download; then \
    ok=1; \
    break; \
    fi; \
    echo "go mod download retry ${i}"; \
    done; \
    [ "$ok" -eq 1 ]

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/server ./cmd/server

# Final stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/bin/server .
COPY --from=builder /app/config/config.yaml ./config/config.yaml

EXPOSE 8080

CMD ["./server"]
