FROM golang:1.25-alpine AS builder

# git is required for Go's VCS build-info stamping (embeds version/revision
# into the binary); alpine doesn't ship it by default.
RUN apk add --no-cache git

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o homelink ./cmd/homelink

# ── runtime ──────────────────────────────────────────────────────────────────
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/homelink /homelink

EXPOSE 8080

ENTRYPOINT ["/homelink"]
