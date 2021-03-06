#!/bin/sh

# Figure out where we are.
Dir=$(dirname -- $(readlink -f -- "$0"))
Build="${Dir}/../.."

docker build -f - -t "mux:latest" "$Build" <<EOF
FROM golang AS builder

WORKDIR /build/summit
COPY . .

WORKDIR /build/summit/cmd/mux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s"

FROM ubuntu:latest

RUN apt-get update && apt-get install -y vim

WORKDIR /build/summit/cmd/mux
COPY --from=builder /build/summit/cmd/mux/mux .

CMD ["/build/summit/cmd/mux/mux", "-l", "docker", "/bin/bash"]
EOF
