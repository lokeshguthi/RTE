#!/usr/bin/env bash
set -e
trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT
go get
go build
./rte-go -host localhost -port 8681 -basedir live/ 1>./server.log 2>./server.err.log &
if [ -z "$1" ]; then
    ./rte-go -host localhost -port 8681 -basedir live/ -testSolution &
else
    ./rte-go -host localhost -port 8681 -basedir live/ -testSolution -testName $1 &
fi
wait -n
pkill -P $$
echo "done"
