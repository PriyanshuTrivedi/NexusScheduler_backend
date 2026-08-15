// module.go wires the Resource service's dependency graph. Config loads, the
// Postgres pool, the Redis client, store, client, controller, handler, and
// the gRPC server bootstrap are each provided declaratively; fx resolves the
// graph and constructs everything in the correct order at startup (Section
// 9.3). Lives in package main alongside main.go, mirroring Booking's flat
// per-service layout — module.go and main.go are the two files in code/
// booking/ (and here, code/resource/) that are not inside their own layer
// subfolder.
package config

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"google.golang.org/grpc"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/controller"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/handler"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/store"
	resourcepb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/configloader"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/grpcserver"
)

// Config is resource's environment-scoped configuration (dev/staging/
// production yaml). Secrets — the Postgres DSN and Redis address — are never
// read from these files; they come from the environment (Section 9.4).
type Config struct {
	GRPCPort int    `yaml:"grpc_port"`
	Env      string `yaml:"env"`
}

func loadConfig() (Config, error) {
	var cfg Config
	if err := configloader.Load("resource", &cfg); err != nil {
		return Config{}, fmt.Errorf("resource: load config: %w", err)
	}
	return cfg, nil
}

// grpcServerConfig maps this service's own Config down to the minimal shape
// grpcserver.Start needs — keeps grpcserver from having to import every
// service's Config type just to read one field out of it.
func grpcServerConfig(cfg Config) grpcserver.Config {
	return grpcserver.Config{Port: cfg.GRPCPort}
}

func newPostgresPool() (*pgxpool.Pool, error) {
	dsn := configloader.MustGetEnv("RESOURCE_POSTGRES_DSN")
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("resource: connect postgres: %w", err)
	}
	return pool, nil
}

func newRedisClient() *redis.Client {
	addr := configloader.MustGetEnv("REDIS_ADDR")
	return redis.NewClient(&redis.Options{Addr: addr})
}

func registerHandler(server *grpc.Server, h *handler.Handler) {
	resourcepb.RegisterResourceServiceServer(server, h)
}

var Module = fx.Options(
	fx.Provide(loadConfig),
	fx.Provide(grpcServerConfig),
	fx.Provide(newPostgresPool),
	fx.Provide(newRedisClient),
	fx.Provide(store.New),
	fx.Provide(client.New),
	fx.Provide(controller.New),
	fx.Provide(handler.New),
	grpcserver.Module,
	fx.Invoke(registerHandler),
	fx.Invoke(grpcserver.Start),
)
