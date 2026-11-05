# syntax=docker/dockerfile:1

# -------------------------------------------------
# Etapa de build
# -------------------------------------------------
FROM golang:1.27-alpine AS builder
WORKDIR /app

# Dependencias del sistema para compilar (git para módulos privados, ca-certificates para builds seguros)
RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/api ./cmd/api

# -------------------------------------------------
# Etapa de ejecución (imagen mínima, sin root)
# -------------------------------------------------
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /

# Certificados y zona horaria para llamadas HTTPS y logs con fecha correcta
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /bin/api /bin/api

USER nonroot:nonroot

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --retries=3 --start-period=10s \
  CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/bin/api"]
