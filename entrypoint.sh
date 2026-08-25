#!/bin/sh
set -e

# All inter-service addresses in code/*/config/*.yaml already point at
# "localhost:<port>" (9002/9003/9004) and redis_addr: localhost:6379 —
# that's true whether they're separate containers on a Docker network or,
# as here, separate processes sharing one container's loopback interface.
# So nothing in the Go code has to change; we just have to make sure every
# process is actually up before api-gateway starts dialing them.

cleanup() {
  echo "shutting down..."
  kill $(jobs -p) 2>/dev/null
  wait
}
trap cleanup TERM INT

echo "starting redis..."
redis-server --daemonize no --bind 127.0.0.1 --port 6379 &
REDIS_ADDR="localhost:6379"
export REDIS_ADDR

echo "starting identity..."
/app/identity &

echo "starting resource..."
/app/resource &

echo "starting booking..."
/app/booking &

# Crude but effective: give the gRPC servers a moment to bind their ports
# before api-gateway's fx startup tries to dial them. All 4 processes log
# to the same stdout, so this also keeps the log ordering readable.
sleep 3

echo "starting api-gateway (foreground)..."
# Run api-gateway as the foreground process so it receives signals directly
# and its exit ends the container, while backgrounded services above are
# cleaned up by the trap.
/app/api-gateway &
API_PID=$!
wait $API_PID