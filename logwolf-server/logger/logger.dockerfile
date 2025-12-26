FROM golang:1.25.4-alpine as builder

RUN mkdir /app
COPY . /app

WORKDIR /app

WORKDIR ./logger
RUN CGO_ENABLED=0 go build -o logger ./cmd/api

RUN chmod +x /app/logger

FROM alpine:latest

RUN mkdir /app

COPY --from=builder /app/logger /app

CMD [ "/app/logger" ]