#!/bin/sh

golangci-lint run --sort-results | grep -Fv TODO
