// module.go wires the booking service's dependency graph. Config loads, the
// Postgres pool, the Redis client, store, client, controller, handler, and
// the gRPC server bootstrap are each provided declaratively; fx resolves the
// graph and constructs everything in the correct order at startup (Section
// 9.3). Lives in package main alongside main.go, mirroring Booking's flat
// per-service layout — module.go and main.go are the two files in code/
// booking/ (and here, code/booking/) that are not inside their own layer
// subfolder.
package config

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"google.golang.org/grpc"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/controller"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/handler"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/mailer"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/store"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/configloader"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/grpcserver"
)

// Config is booking's environment-scoped configuration (dev/staging/
// production yaml). Secrets — the Postgres DSN and Redis address — are never
// read from these files; they come from the environment (Section 9.4).
type Config struct {
	GRPCPort int        `yaml:"grpc_port"`
	Env      string     `yaml:"env"`
	Mail     MailConfig `yaml:"mail"`
}

type MailConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	From     string `yaml:"from"`
}

func loadConfig() (Config, error) {
	var cfg Config
	if err := configloader.Load("booking", &cfg); err != nil {
		return Config{}, fmt.Errorf("booking: load config: %w", err)
	}
	return cfg, nil
}

func newPostgresPool() (*pgxpool.Pool, error) {
	dsn := configloader.MustGetEnv("BOOKING_POSTGRES_DSN")
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("booking: connect postgres: %w", err)
	}
	return pool, nil
}

func newRedisClient() *redis.Client {
	addr := configloader.MustGetEnv("REDIS_ADDR")
	return redis.NewClient(&redis.Options{Addr: addr})
}

func grpcServerConfig(cfg Config) grpcserver.Config {
	return grpcserver.Config{Port: cfg.GRPCPort}
}

func newMailer(cfg Config) mailer.Mailer {
	return mailer.NewSMTP(mailer.Config{
		Host: cfg.Mail.Host, Port: cfg.Mail.Port, Username: cfg.Mail.Username,
		Password: os.Getenv("BOOKING_SMTP_PASSWORD"), From: cfg.Mail.From,
	})
}

func registerHandler(server *grpc.Server, h *handler.Handler) {
	bookingpb.RegisterBookingServiceServer(server, h)
}

var Module = fx.Options(
	fx.Provide(loadConfig),
	fx.Provide(newPostgresPool),
	fx.Provide(newRedisClient),
	fx.Provide(store.New),
	fx.Provide(newMailer),
	fx.Provide(client.New),
	fx.Provide(controller.New),
	fx.Provide(handler.New),
	fx.Provide(grpcServerConfig),
	grpcserver.Module,
	fx.Invoke(registerHandler),
	fx.Invoke(grpcserver.Start),
)
