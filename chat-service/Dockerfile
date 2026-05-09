FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod ./
COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /chat-service ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /chat-service .
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/deploy/migrations ./deploy/migrations

EXPOSE 8083

CMD ["./chat-service"]
