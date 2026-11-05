# ---- Build stage ----
FROM golang:1.25-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Generate protobuf code because gen/ is gitignored.
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10 \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

ENV PATH="/root/go/bin:${PATH}"

RUN mkdir -p gen && protoc --proto_path=. \
    --go_out=paths=source_relative:gen \
    --go-grpc_out=paths=source_relative:gen \
    idl/resource/resource.proto \
    idl/booking/booking.proto \
    idl/identity/identity.proto \
    idl/chatbot/chatbot.proto

# Build static binaries.
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

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/ /app/

# Runtime configs for all services.
COPY --from=builder /src/code/identity/config /app/code/identity/config
COPY --from=builder /src/code/resource/config /app/code/resource/config
COPY --from=builder /src/code/booking/config /app/code/booking/config
COPY --from=builder /src/code/chatbot/config /app/code/chatbot/config
COPY --from=builder /src/code/mcp/config /app/code/mcp/config
COPY --from=builder /src/code/api-gateway/config /app/code/api-gateway/config

COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

ENV ENV=production

EXPOSE 10000 9002 9003 9004 9005 9006

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["api-gateway"]