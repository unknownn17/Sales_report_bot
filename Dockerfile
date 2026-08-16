FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/telegram-sales-bot ./cmd

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /out/telegram-sales-bot /app/telegram-sales-bot

ENV BOT_TOKEN=""
ENV ADMIN_IDS=""
ENV MONGO_URI=""
ENV DB_NAME=""
ENTRYPOINT ["/app/telegram-sales-bot"]
