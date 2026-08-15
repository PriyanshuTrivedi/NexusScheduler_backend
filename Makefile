.PHONY: gazelle build test run-% mocks proto up down logs migrate-up migrate-down

ifneq (,$(wildcard .env))
include .env
export
endif

gazelle:
	bazel run //:gazelle

build:
	bazel build //...

test:
	bazel test //...

run-%:
	bazel run //code/$*:$*

proto:
	protoc --proto_path=. --go_out=paths=source_relative:gen --go-grpc_out=paths=source_relative:gen idl/resource/resource.proto idl/booking/booking.proto idl/identity/identity.proto

mocks:
	go generate ./...
	bazel run //:gazelle

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f
migrate-up:
	migrate -path code/identity/db -database "$(IDENTITY_POSTGRES_DSN)" up
	migrate -path code/resource/db -database "$(RESOURCE_POSTGRES_DSN)" up
	migrate -path code/booking/db -database "$(BOOKING_POSTGRES_DSN)" up

migrate-down:
	migrate -path code/booking/db -database "$(BOOKING_POSTGRES_DSN)" down -all
	migrate -path code/resource/db -database "$(RESOURCE_POSTGRES_DSN)" down -all
	migrate -path code/identity/db -database "$(IDENTITY_POSTGRES_DSN)" down -all
