#!/bin/bash

set -e

echo "Duration	Latency	Proto"
go build -o grpcbench
for latency in 32; do
	for grpc in true false; do
		args="-latency=${latency}ms -grpc=$grpc"
		echo
		./grpcbench $args
	done
done
