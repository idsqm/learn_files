package config

import (
	"context"
	"fmt"
	"time"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	ServerPort       string        `env:"SERVER_PORT,default=8082"`
	DBURL            string        `env:"DATABASE_URL,required"`
	JWTSecret        string        `env:"JWT_SECRET,required"`
	Debug            bool          `env:"DEBUG,default=false"`
	S3Endpoint       string        `env:"S3_ENDPOINT,required"`
	S3PublicEndpoint string        `env:"S3_PUBLIC_ENDPOINT,default="`
	S3Region         string        `env:"S3_REGION,default=us-east-1"`
	S3Bucket         string        `env:"S3_BUCKET,required"`
	S3AccessKey      string        `env:"S3_ACCESS_KEY,required"`
	S3SecretKey      string        `env:"S3_SECRET_KEY,required"`
	S3UsePathStyle   bool          `env:"S3_USE_PATH_STYLE,default=true"`
	PresignedTTL     time.Duration `env:"PRESIGNED_URL_TTL,default=1h"`
	PendingFileTTL   time.Duration `env:"PENDING_FILE_TTL,default=24h"`
	CORSOrigins      []string      `env:"CORS_ORIGINS,default=http://localhost:5173"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return &cfg, nil
}
