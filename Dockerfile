# ISPSYNC Nexus (ISP Onboarding Gateway)
# Multi-stage production Dockerfile

FROM golang:alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/nexus ./cmd/api

FROM alpine:3.20 AS production

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/nexus /app/nexus
COPY web /app/web

EXPOSE 8081

HEALTHCHECK --interval=15s --timeout=3s --retries=3 \
  CMD curl -fs http://127.0.0.1:8081/health || exit 1

ENTRYPOINT ["/app/nexus"]
