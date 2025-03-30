FROM golang:1.23-alpine as builder

RUN apk add --no-cache \
    pkgconf \
    openssl-dev \
    gcc \
    musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /app/my-mattermost-bot ./cmd/my-mattermost-bot

FROM alpine:latest


WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/my-mattermost-bot .
COPY --from=builder /app/bot.env .

RUN ls -la /app && \
    chmod +x /app/my-mattermost-bot

CMD ["/app/my-mattermost-bot"]
