#!/usr/bin/env bash

container_name="plugin-cosmos.wasmd"
container_version="v0.40.1"
genesis_account="wasm1lsagfzrm4gz28he4wunt63sts5xzmczwda8vl6"

set -euox pipefail

bash "$(dirname -- "$0")/wasmd.down.sh"

listen_ips=""
if [ "$(uname)" = "Darwin" ]; then
	echo "Listening on all interfaces on MacOS"
	listen_ips="0.0.0.0"
else
	docker_ip=$(docker network inspect bridge -f '{{range .IPAM.Config}}{{.Gateway}}{{end}}')
	if [ -z "${docker_ip}" ]; then
		echo "Could not fetch docker ip."
		exit 1
	fi
	listen_ips="127.0.0.1 ${docker_ip}"
fi

echo "Starting wasmd container"

listen_args=()
for ip in $listen_ips; do
	listen_args+=("-p" "${ip}:26657:26657")
done

docker run \
	"${listen_args[@]}" \
	-d \
	--platform linux/amd64 \
	--name "${container_name}" \
	"cosmwasm/wasmd:${container_version}" \
	"./setup_and_run.sh" \
	"${genesis_account}"

echo "Waiting for wasmd container to become ready.."
start_time=$(date +%s)
prev_output=""
while true; do
	output=$(docker logs "${container_name}" 2>&1)
	if [[ "${output}" != "${prev_output}" ]]; then
		echo -n "${output#$prev_output}"
		prev_output="${output}"
	fi

	if [[ $output == *"Replay: Done"* ]]; then
		echo ""
		echo "wasmd is ready."
		exit 0
	fi

	current_time=$(date +%s)
	elapsed_time=$((current_time - start_time))

	if ((elapsed_time > 600)); then
		echo "Error: Command did not become ready within 600 seconds"
		exit 1
	fi

	sleep 3
done
