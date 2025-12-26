FROM golang:1.25.4-alpine as builder

RUN mkdir /app
COPY . /app

WORKDIR /app

WORKDIR ./broker
RUN CGO_ENABLED=0 go build -o broker ./cmd/api

RUN chmod +x /app/broker

FROM alpine:latest

RUN mkdir /app

COPY --from=builder /app/broker /app

CMD [ "/app/broker" ]