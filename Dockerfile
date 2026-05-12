# phase 1 - build
ARG GO_VERSION
FROM golang:${GO_VERSION:-1.24}-alpine AS builder

WORKDIR /app

RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bot .

# phase 2 - run
FROM alpine:latest

# baked-in identity for /status + JSON log fields per memo-docs/standards/observability.md.
# CI passes these as --build-arg; buildinfo.go reads them via env at startup.
ARG MEMO_VERSION=dev
ARG MEMO_BUILD=unknown
ENV MEMO_VERSION=${MEMO_VERSION} \
    MEMO_BUILD=${MEMO_BUILD}

RUN apk --no-cache add ca-certificates

COPY --from=builder /bot /bot

# 健康检查端口
EXPOSE 8080

CMD ["/bot"]