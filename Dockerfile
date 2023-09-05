FROM golang:1.21 as builder
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./
RUN go build -o /git-mirror

FROM alpine/git
RUN apk add --no-cache libc6-compat
WORKDIR /
COPY --from=builder /git-mirror git-mirror
ENTRYPOINT [ "/git-mirror" ]
