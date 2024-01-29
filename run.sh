#!/usr/bin/env bash
set -e
go get
go build
./rte-go -host localhost -port 8081 -basedir live/