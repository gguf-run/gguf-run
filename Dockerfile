FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /gguf-run .

FROM alpine:edge

RUN apk add --no-cache llama.cpp ca-certificates

COPY --from=builder /gguf-run /usr/local/bin/gguf-run

ENTRYPOINT ["gguf-run"]
