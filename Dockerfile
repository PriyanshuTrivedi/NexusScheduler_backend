# ---- Build stage ----
FROM golang:1-bookworm AS builder

# protoc + Go plugins, needed because gen/ (proto output) is .gitignored
# and isn't committed to the repo — it has to be regenerated at build time.
RUN apt-get update && apt-get install -y --no-install-recommends protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10 \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

ENV PATH="/root/go/bin:${PATH}"

RUN mkdir -p gen && protoc --proto_path=. \
    --go_out=paths=source_relative:gen --go-grpc_out=paths=source_relative:gen \
    idl/resource/resource.proto \
    idl/booking/booking.proto \
    idl/identity/identity.proto \
    idl/chatbot/chatbot.proto

# Static binaries — no CGO, so the runtime stage can use a minimal image.
ENV CGO_ENABLED=0

RUN mkdir -p /out \
    && go build -tags netgo -ldflags '-s -w' -o /out/identity ./code/identity \
    && go build -tags netgo -ldflags '-s -w' -o /out/resource ./code/resource \
    && go build -tags netgo -ldflags '-s -w' -o /out/booking ./code/booking \
    && go build -tags netgo -ldflags '-s -w' -o /out/chatbot ./code/chatbot \
    && go build -tags netgo -ldflags '-s -w' -o /out/mcp ./code/mcp \
    && go build -tags netgo -ldflags '-s -w' -o /out/api-gateway ./code/api-gateway

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache redis ca-certificates tzdata

WORKDIR /app

# Binaries (including the new MCP server)
COPY --from=builder /out/ /app/

# Runtime configs
COPY --from=builder /src/code/identity/config /app/code/identity/config
COPY --from=builder /src/code/resource/config /app/code/resource/config
COPY --from=builder /src/code/booking/config /app/code/booking/config
COPY --from=builder /src/code/chatbot/config /app/code/chatbot/config
COPY --from=builder /src/code/mcp/config /app/code/mcp/config
COPY --from=builder /src/code/api-gateway/config /app/code/api-gateway/config

COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

ENV ENV=production

# API Gateway + internal gRPC services + MCP HTTP server.
EXPOSE 8080 9002 9003 9004 9005 9006

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["api-gateway"]