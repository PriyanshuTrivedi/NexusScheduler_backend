package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"go.uber.org/fx"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/mcp/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/configloader"
)

type Config struct {
	HTTPPort              int    `yaml:"http_port"`
	ListenHost            string `yaml:"listen_host"`
	APIGatewayBaseURL     string `yaml:"api_gateway_base_url"`
	RequestTimeoutSeconds int    `yaml:"request_timeout_seconds"`
}

func loadConfig() (Config, error) {
	var cfg Config

	if err := configloader.Load("mcp", &cfg); err != nil {
		return Config{}, fmt.Errorf("mcp: load config: %w", err)
	}

	if value := os.Getenv("API_GATEWAY_BASE_URL"); value != "" {
		cfg.APIGatewayBaseURL = value
	}

	if value := os.Getenv("MCP_HTTP_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port <= 0 {
			return Config{}, fmt.Errorf("mcp: invalid MCP_HTTP_PORT: %q", value)
		}
		cfg.HTTPPort = port
	}

	if cfg.HTTPPort <= 0 {
		cfg.HTTPPort = 9006
	}

	if cfg.ListenHost == "" {
		cfg.ListenHost = "0.0.0.0"
	}

	if cfg.RequestTimeoutSeconds <= 0 {
		cfg.RequestTimeoutSeconds = 15
	}

	if cfg.APIGatewayBaseURL == "" {
		return Config{}, fmt.Errorf("mcp: API_GATEWAY_BASE_URL is required")
	}

	return cfg, nil
}

func newAPIGatewayClient(cfg Config) (client.APIGatewayClient, error) {
	return client.NewAPIGatewayClient(
		cfg.APIGatewayBaseURL,
		time.Duration(cfg.RequestTimeoutSeconds)*time.Second,
	)
}

var Module = fx.Options(
	fx.Provide(loadConfig),
	fx.Provide(newAPIGatewayClient),
)
