package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Log         LogConfig         `mapstructure:"log"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Redis       RedisConfig       `mapstructure:"redis"`
	RabbitMQ    RabbitMQConfig    `mapstructure:"rabbitmq"`
	Meilisearch MeilisearchConfig `mapstructure:"meilisearch"`
	Vectorizer  VectorizerConfig  `mapstructure:"vectorizer"`
	JWT         JWTConfig         `mapstructure:"jwt"`
}

type ServerConfig struct {
	Addr string `mapstructure:"addr"`
	Mode string `mapstructure:"mode"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
	Dir   string `mapstructure:"dir"`
}

type DatabaseConfig struct {
	DSN             string `mapstructure:"dsn"`
	MaxIdle         int    `mapstructure:"max_idle"`
	MaxOpen         int    `mapstructure:"max_open"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"` // seconds
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type RabbitMQConfig struct {
	URL string `mapstructure:"url"`
}

type MeilisearchConfig struct {
	Addr   string `mapstructure:"addr"`
	APIKey string `mapstructure:"api_key"`
	Index  string `mapstructure:"index"`
}

type VectorizerConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	Model     string `mapstructure:"model"`
	TimeoutMS int    `mapstructure:"timeout_ms"`
}

type JWTConfig struct {
	PrivateKeyPath string `mapstructure:"private_key_path"`
	PublicKeyPath  string `mapstructure:"public_key_path"`
	AccessExpire   int    `mapstructure:"access_expire"`  // seconds
	RefreshExpire  int    `mapstructure:"refresh_expire"` // seconds
	Issuer         string `mapstructure:"issuer"`
}

func (j *JWTConfig) AccessDuration() time.Duration {
	return time.Duration(j.AccessExpire) * time.Second
}

func (j *JWTConfig) RefreshDuration() time.Duration {
	return time.Duration(j.RefreshExpire) * time.Second
}

func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Environment variable overrides: CGOFORUM_SERVER_ADDR, CGOFORUM_DATABASE_DSN, etc.
	v.SetEnvPrefix("CGOFORUM")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}
