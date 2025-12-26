FROM golang:1.25.4-alpine as builder

RUN mkdir /app
COPY . /app

WORKDIR /app

WORKDIR ./listener
RUN CGO_ENABLED=0 go build -o listener ./cmd/api

RUN chmod +x /app/listener

FROM alpine:latest

RUN mkdir /app

COPY --from=builder /app/listener /app

CMD [ "/app/listener" ]