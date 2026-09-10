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

echo "waiting for services..."

for i in $(seq 1 30); do
    if nc -z 127.0.0.1 9002 &&
       nc -z 127.0.0.1 9003 &&
       nc -z 127.0.0.1 9004; then
        echo "all gRPC services are ready"
        break
    fi
    sleep 1
done

echo "starting api-gateway..."
/app/api-gateway &
API_PID=$!
wait $API_PID