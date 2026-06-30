# --- Build stage ---
    FROM golang:1.22-alpine AS builder

    RUN apk add --no-cache ca-certificates
    
    WORKDIR /app
    COPY go.mod ./
    COPY *.go ./
    
    RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server-monitor .
    
    # --- Final stage ---
    FROM scratch
    
    # CA certs are needed for TLS verification (Go's crypto/tls dial) and for HTTPS to Discord.
    COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
    COPY --from=builder /app/server-monitor /server-monitor
    
    ENTRYPOINT ["/server-monitor"]