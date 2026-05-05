FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

# Copy contracts first so the replace directive (../contracts) resolves correctly.
# go.mod contains: replace github.com/mihnpro/DatingBotProtos => ../contracts
# WORKDIR is /app, so ../contracts resolves to /contracts inside the container.
COPY contracts/ /contracts/

COPY media-service/go.mod media-service/go.sum ./
RUN go mod download

COPY media-service/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o /media-service ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /root/

COPY --from=builder /media-service .
COPY --from=builder /app/configs ./configs

EXPOSE 50054 8083

CMD ["./media-service"]
