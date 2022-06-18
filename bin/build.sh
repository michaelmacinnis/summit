#!/bin/sh -e

# Figure out where we are.
Dir=$(dirname -- $(readlink -f -- "$0"))

cd "${Dir}/.."

go build ./cmd/client
go build ./cmd/mux
go build ./cmd/server

sudo cp client /usr/local/bin/summit-client
sudo cp mux /usr/local/bin/summit-mux

./cmd/mux/build.sh
