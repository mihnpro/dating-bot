FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /notification-service ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /root/

COPY --from=builder /notification-service .
COPY --from=builder /app/configs ./configs

EXPOSE 8087

CMD ["./notification-service"]
