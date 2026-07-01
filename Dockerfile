FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /files ./cmd/files

FROM alpine:3.21 AS runner

RUN apk add --no-cache ca-certificates

COPY --from=builder /files /files
COPY --from=builder /app/migrations /migrations

EXPOSE 8082

ENTRYPOINT ["/files"]
