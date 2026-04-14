#!/bin/sh
cd "$(dirname "$0")/client"
CGO_ENABLED=1 go build -o scrunch-client . && ./scrunch-client "$@"
