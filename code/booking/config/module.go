package config

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"google.golang.org/grpc"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/controller"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/handler"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/mailer"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/mailer/smtp"
	mailertemplate "github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/mailer/template"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/store"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/configloader"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/grpcserver"
)

type Config struct {
	GRPCPort int    `yaml:"grpc_port"`
	Env      string `yaml:"env"`
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

	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

func grpcServerConfig(cfg Config) grpcserver.Config {
	return grpcserver.Config{
		Port: cfg.GRPCPort,
	}
}

func newMailer() (mailer.Mailer, error) {
	host := configloader.MustGetEnv("BOOKING_SMTP_HOST")
	portStr := configloader.MustGetEnv("BOOKING_SMTP_PORT")
	username := configloader.MustGetEnv("BOOKING_SMTP_USERNAME")
	password := configloader.MustGetEnv("BOOKING_SMTP_PASSWORD")
	from := configloader.MustGetEnv("BOOKING_SMTP_FROM")

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("booking: invalid BOOKING_SMTP_PORT: %w", err)
	}

	smtpSender := smtp.New(host, port, username, password)
	renderer := mailertemplate.NewRenderer()

	return mailer.New(smtpSender, renderer, from), nil
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
