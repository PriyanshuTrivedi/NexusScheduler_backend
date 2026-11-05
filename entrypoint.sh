#!/bin/sh
set -eu

SERVICE="${1:-api-gateway}"

case "$SERVICE" in
    identity|resource|booking|chatbot|mcp|api-gateway)
        echo "Starting NexusScheduler service: $SERVICE"
        exec "/app/$SERVICE"
        ;;
    *)
        echo "Error: unknown service '$SERVICE'"
        echo "Allowed services: identity, resource, booking, chatbot, mcp, api-gateway"
        exit 1
        ;;
esac