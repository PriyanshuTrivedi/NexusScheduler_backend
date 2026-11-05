package config

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"google.golang.org/grpc"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/client/llmclients"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/controller"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/handler"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/store"
	chatbotpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/configloader"
	"github.com/PriyanshuTrivedi/nexus-scheduler/pkg/grpcserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GeminiLLMConfig struct {
	Model            string `yaml:"model"`
	MaxRetries       int    `yaml:"max_retries"`
	RetryBaseDelayMs int    `yaml:"retry_base_delay_ms"`
}

type MCPClientConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type MCPConfig struct {
	Endpoint string          `yaml:"server_url"`
	Client   MCPClientConfig `yaml:"client"`
}

type Config struct {
	GRPCPort     int             `yaml:"grpc_port"`
	Env          string          `yaml:"env"`
	MCPConfig    MCPConfig       `yaml:"mcp"`
	GeminiConfig GeminiLLMConfig `yaml:"gemini"`
}

func loadConfig() (Config, error) {
	var cfg Config

	if err := configloader.Load("chatbot", &cfg); err != nil {
		return Config{}, fmt.Errorf("chatbot: load config: %w", err)
	}

	if cfg.GeminiConfig.Model == "" {
		cfg.GeminiConfig.Model = "gemini-3.5-flash-lite"
	}
	if cfg.GeminiConfig.MaxRetries < 0 {
		return Config{}, fmt.Errorf("chatbot: max_retries cannot be negative")
	}
	if cfg.GeminiConfig.RetryBaseDelayMs <= 0 {
		cfg.GeminiConfig.RetryBaseDelayMs = 250
	}

	if cfg.MCPConfig.Endpoint == "" {
		cfg.MCPConfig.Endpoint = configloader.MustGetEnv("MCP_SERVER_URL")
	}
	if cfg.MCPConfig.Client.Name == "" {
		return Config{}, fmt.Errorf("chatbot: mcp client name is required")
	}
	if cfg.MCPConfig.Client.Version == "" {
		return Config{}, fmt.Errorf("chatbot: mcp client verison is required")
	}

	return cfg, nil
}

func newPostgresPool() (*pgxpool.Pool, error) {
	dsn := configloader.MustGetEnv("CHATBOT_POSTGRES_DSN")

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("chatbot: connect postgres: %w", err)
	}

	return pool, nil
}

func newLLM(cfg Config) (client.LLM, error) {
	apiKey := configloader.MustGetEnv("GEMINI_LLM_API_KEY")

	llm, err := llmclients.NewGeminiClient(
		context.Background(),
		apiKey,
		cfg.GeminiConfig.Model,
	)
	if err != nil {
		return nil, fmt.Errorf("chatbot: initialize Gemini client: %w", err)
	}

	return llm, nil
}

func newMCPClient(cfg Config) client.MCPClient {
	return client.NewMCPClient(cfg.MCPConfig.Endpoint, mcp.NewClient(
		&mcp.Implementation{
			Name:    cfg.MCPConfig.Client.Name,
			Version: cfg.MCPConfig.Client.Version,
		},
		nil,
	))
}

func grpcServerConfig(cfg Config) grpcserver.Config {
	return grpcserver.Config{
		Port: cfg.GRPCPort,
	}
}

func registerHandler(server *grpc.Server, h *handler.Handler) {
	chatbotpb.RegisterChatbotServiceServer(server, h)
}

func newController(st store.Store, llm client.LLM, mcp client.MCPClient, cfg Config) controller.Controller {
	return controller.New(st, llm, mcp, controller.RetryConfig{
		MaxRetries: cfg.GeminiConfig.MaxRetries,
		BaseDelay:  time.Duration(cfg.GeminiConfig.RetryBaseDelayMs) * time.Millisecond,
	})
}

var Module = fx.Options(
	fx.Provide(loadConfig),
	fx.Provide(newPostgresPool),
	fx.Provide(store.New),
	fx.Provide(newLLM),
	fx.Provide(newMCPClient),
	fx.Provide(newController),
	fx.Provide(handler.New),
	fx.Provide(grpcServerConfig),
	grpcserver.Module,
	fx.Invoke(registerHandler),
	fx.Invoke(grpcserver.Start),
)
