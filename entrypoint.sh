#!/bin/sh
set -e

cleanup() {
  echo "shutting down..."
  kill $(jobs -p) 2>/dev/null || true
  wait || true
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

echo "waiting for core services..."
for i in $(seq 1 30); do
    if nc -z 127.0.0.1 9002 &&
       nc -z 127.0.0.1 9003 &&
       nc -z 127.0.0.1 9004; then
        echo "core services are ready"
        break
    fi
    sleep 1
done

echo "starting mcp..."
/app/mcp &

echo "waiting for MCP..."
for i in $(seq 1 30); do
    if nc -z 127.0.0.1 9006; then
        echo "MCP is ready"
        break
    fi
    sleep 1
done

echo "starting chatbot..."
/app/chatbot &

echo "waiting for chatbot..."
for i in $(seq 1 30); do
    if nc -z 127.0.0.1 9005; then
        echo "all gRPC services are ready"
        break
    fi
    sleep 1
done

echo "starting api-gateway..."
/app/api-gateway &
API_PID=$!
wait $API_PID
