#!/bin/bash

set -e

echo "Duration	Latency	Proto"
go build -o grpcbench
for latency in 0 1 2 4 8 16 32; do
	for grpc in true false; do
		for http2 in true false; do
			if [[ "$grpc" == "true" && "$http2" == "false" ]]; then
				continue
			fi
			echo
			./grpcbench -latency=${latency}ms -grpc=$grpc -http2=$http2 -n 5
		done
	done
done
