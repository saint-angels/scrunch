#!/bin/sh
cd "$(dirname "$0")/server"
go build -o scrunch-server . && ./scrunch-server "$@"
