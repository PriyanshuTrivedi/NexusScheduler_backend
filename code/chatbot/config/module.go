package config

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"google.golang.org/grpc"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/chunker"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/chunker/techniques"
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

type PostgresConfig struct {
	MaxConns               int `yaml:"max_conns"`
	MinConns               int `yaml:"min_conns"`
	ConnMaxLifetimeMinutes int `yaml:"conn_max_lifetime_minutes"`
}

type GeminiLLMConfig struct {
	Model string `yaml:"model"`
}

type GeminiEmbeddingConfig struct {
	Model      string `yaml:"model"`
	Dimensions int32  `yaml:"dimensions"`
}

type GeminiConfig struct {
	LLMConfig        GeminiLLMConfig       `yaml:"llm"`
	EmbeddingConfig  GeminiEmbeddingConfig `yaml:"embedding"`
	MaxRetries       int                   `yaml:"max_retries"`
	RetryBaseDelayMs int                   `yaml:"retry_base_delay_ms"`
}

type PineconeConfig struct {
	IndexHost string `yaml:"index_host"`
}

type FixedSizeChunkingConfig struct {
	ChunkSize    int `yaml:"chunk_size"`
	Overlap      int `yaml:"overlap"`
	MinChunkSize int `yaml:"min_chunk_size"`
}

type ChunkingConfig struct {
	Fixed FixedSizeChunkingConfig `yaml:"strategy_fixed"`
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
	Env            string         `yaml:"env"`
	LogLevel       string         `yaml:"log_level"`
	GRPCPort       int            `yaml:"grpc_port"`
	PostgresConfig PostgresConfig `yaml:"postgres"`
	GeminiConfig   GeminiConfig   `yaml:"gemini"`
	PineconeConfig PineconeConfig `yaml:"pinecone"`
	MCPConfig      MCPConfig      `yaml:"mcp"`
	ChunkingConfig ChunkingConfig `yaml:"chunking"`
}

func loadConfig() (Config, error) {
	var cfg Config

	if err := configloader.Load("chatbot", &cfg); err != nil {
		return Config{}, fmt.Errorf("chatbot: load config: %w", err)
	}

	// Gemini LLM configs
	if cfg.GeminiConfig.LLMConfig.Model == "" {
		cfg.GeminiConfig.LLMConfig.Model = "gemini-3.5-flash-lite"
	}
	if cfg.GeminiConfig.RetryBaseDelayMs <= 0 {
		cfg.GeminiConfig.RetryBaseDelayMs = 250
	}
	if cfg.GeminiConfig.MaxRetries < 0 {
		return Config{}, fmt.Errorf("chatbot: gemini max_retries cannot be negative")
	}

	// Gemini embedding configs
	if cfg.GeminiConfig.EmbeddingConfig.Model == "" {
		cfg.GeminiConfig.EmbeddingConfig.Model = "gemini-embedding-001"
	}
	if cfg.GeminiConfig.EmbeddingConfig.Dimensions <= 0 {
		return Config{}, fmt.Errorf("chatbot: embedding dimensions must be positive")
	}

	// Pinecone configs
	if cfg.PineconeConfig.IndexHost == "" {
		return Config{}, fmt.Errorf("chatbot: pinecone index_host is required")
	}

	// MCP configs
	if endpoint := os.Getenv("MCP_SERVER_URL"); endpoint != "" {
		cfg.MCPConfig.Endpoint = endpoint
	}
	if cfg.MCPConfig.Client.Name == "" {
		return Config{}, fmt.Errorf("chatbot: mcp client name is required")
	}
	if cfg.MCPConfig.Client.Version == "" {
		return Config{}, fmt.Errorf("chatbot: mcp client version is required")
	}

	// PostgreSQL configs
	if cfg.PostgresConfig.MaxConns <= 0 {
		return Config{}, fmt.Errorf("chatbot: postgres max_conns must be positive")
	}
	if cfg.PostgresConfig.MinConns < 0 ||
		cfg.PostgresConfig.MinConns > cfg.PostgresConfig.MaxConns {
		return Config{}, fmt.Errorf("chatbot: postgres min_conns must be between 0 and max_conns")
	}
	if cfg.PostgresConfig.ConnMaxLifetimeMinutes <= 0 {
		return Config{}, fmt.Errorf("chatbot: postgres conn_max_lifetime_minutes must be positive")
	}

	// Chunking configs
	chunking := cfg.ChunkingConfig.Fixed
	if chunking.ChunkSize <= 0 {
		return Config{}, fmt.Errorf("chatbot: chunking chunk_size must be positive")
	}
	if chunking.Overlap < 0 || chunking.Overlap >= chunking.ChunkSize {
		return Config{}, fmt.Errorf("chatbot: chunking overlap must be non-negative and smaller than chunk_size")
	}
	if chunking.MinChunkSize <= 0 || chunking.MinChunkSize > chunking.ChunkSize {
		return Config{}, fmt.Errorf("chatbot: chunking min_chunk_size must be positive and no greater than chunk_size")
	}

	return cfg, nil
}

func newPostgresPool(cfg Config) (*pgxpool.Pool, error) {
	dsn := configloader.MustGetEnv("CHATBOT_POSTGRES_DSN")

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("chatbot: parse postgres config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.PostgresConfig.MaxConns)
	poolConfig.MinConns = int32(cfg.PostgresConfig.MinConns)
	poolConfig.MaxConnLifetime = time.Duration(cfg.PostgresConfig.ConnMaxLifetimeMinutes) * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
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
		llmclients.GeminiConfig{
			Model:               cfg.GeminiConfig.LLMConfig.Model,
			EmbeddingModel:      cfg.GeminiConfig.EmbeddingConfig.Model,
			EmbeddingDimensions: cfg.GeminiConfig.EmbeddingConfig.Dimensions,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("chatbot: initialize Gemini client: %w", err)
	}

	return llm, nil
}

func newPineconeClient(cfg Config) (client.PineconeClient, error) {
	pineconeClient, err := client.NewPineconeClient(client.PineconeConfig{
		APIKey:    configloader.MustGetEnv("PINECONE_API_KEY"),
		IndexHost: cfg.PineconeConfig.IndexHost,
	})
	if err != nil {
		return nil, fmt.Errorf("chatbot: initialize Pinecone client: %w", err)
	}

	return pineconeClient, nil
}

func newChunker(cfg Config) (chunker.Chunker, error) {
	return techniques.NewFixedSizeChunker(techniques.FixedSizeChunkerConfig{
		ChunkSize:    cfg.ChunkingConfig.Fixed.ChunkSize,
		Overlap:      cfg.ChunkingConfig.Fixed.Overlap,
		MinChunkSize: cfg.ChunkingConfig.Fixed.MinChunkSize,
	})
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

func newController(st store.Store, llm client.LLM, mcp client.MCPClient, pinecone client.PineconeClient, chunker chunker.Chunker, cfg Config) controller.Controller {
	return controller.New(st, llm, mcp, pinecone, chunker, controller.RetryConfig{
		MaxRetries: cfg.GeminiConfig.MaxRetries,
		BaseDelay:  time.Duration(cfg.GeminiConfig.RetryBaseDelayMs) * time.Millisecond,
	})
}

var Module = fx.Options(
	fx.Provide(loadConfig),
	fx.Provide(newPostgresPool),
	fx.Provide(store.New),
	fx.Provide(newLLM),
	fx.Provide(newPineconeClient),
	fx.Provide(newChunker),
	fx.Provide(newMCPClient),
	fx.Provide(newController),
	fx.Provide(handler.New),
	fx.Provide(grpcServerConfig),
	grpcserver.Module,
	fx.Invoke(registerHandler),
	fx.Invoke(grpcserver.Start),
)
