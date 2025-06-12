FROM golang:1.23 AS builder
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./
RUN go build -o /git-mirror

FROM alpine:latest
RUN apk add --no-cache git git-daemon libc6-compat
WORKDIR /
COPY --from=builder /git-mirror git-mirror
ENTRYPOINT [ "/git-mirror" ]
