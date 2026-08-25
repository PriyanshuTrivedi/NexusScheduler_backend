# ---- Build stage ----
FROM golang:1.25-bookworm AS builder

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
    idl/resource/resource.proto idl/booking/booking.proto idl/identity/identity.proto

# Static binaries — no CGO, so the runtime stage can be a minimal image.
ENV CGO_ENABLED=0
RUN go build -tags netgo -ldflags '-s -w' -o /out/identity    ./code/identity
RUN go build -tags netgo -ldflags '-s -w' -o /out/resource    ./code/resource
RUN go build -tags netgo -ldflags '-s -w' -o /out/booking     ./code/booking
RUN go build -tags netgo -ldflags '-s -w' -o /out/api-gateway ./code/api-gateway

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache redis ca-certificates

WORKDIR /app

# Binaries
COPY --from=builder /out/identity    /app/identity
COPY --from=builder /out/resource    /app/resource
COPY --from=builder /out/booking     /app/booking
COPY --from=builder /out/api-gateway /app/api-gateway

# configloader reads code/<service>/config/<env>.yaml relative to the
# working directory at runtime, so that folder layout has to exist in the
# final image too — copy just the yaml configs, not the whole source tree.
COPY --from=builder /src/code/identity/config    /app/code/identity/config
COPY --from=builder /src/code/resource/config    /app/code/resource/config
COPY --from=builder /src/code/booking/config     /app/code/booking/config
COPY --from=builder /src/code/api-gateway/config /app/code/api-gateway/config

COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

ENV ENV=production
# api-gateway's yaml already hardcodes http_port: 8080 — tell HF Spaces to
# route its single public port there (set app_port: 8080 in the Space
# README frontmatter) instead of changing this.
EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]