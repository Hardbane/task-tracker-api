package config

import (
	"flag"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App       AppConfig       `yaml:"app"`
	JWT       JWTConfig       `yaml:"jwt"`
	MySQL     MySQLConfig     `yaml:"mysql"`
	Redis     RedisConfig     `yaml:"redis"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Email     EmailConfig     `yaml:"email"`
}

type AppConfig struct {
	Env             string        `yaml:"env"`
	Address         string        `yaml:"address"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type JWTConfig struct {
	Secret string        `yaml:"secret"`
	TTL    time.Duration `yaml:"ttl"`
}

type MySQLConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string        `yaml:"addr"`
	Password string        `yaml:"password"`
	DB       int           `yaml:"db"`
	TTL      time.Duration `yaml:"ttl"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
}

type EmailConfig struct {
	MockLatency time.Duration `yaml:"mock_latency"`
}

func Load() (Config, error) {
	path := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg := defaultConfig()
	if data, err := os.ReadFile(*path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	}

	overrideFromEnv(&cfg)
	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		App:       AppConfig{Env: "local", Address: ":8080", ShutdownTimeout: 10 * time.Second},
		JWT:       JWTConfig{Secret: "change-me", TTL: 24 * time.Hour},
		MySQL:     MySQLConfig{DSN: "task_user:task_password@tcp(localhost:3306)/task_db?parseTime=true&multiStatements=true&charset=utf8mb4&loc=Local", MaxOpenConns: 25, MaxIdleConns: 10, ConnMaxLifetime: 5 * time.Minute},
		Redis:     RedisConfig{Addr: "localhost:6379", TTL: 5 * time.Minute},
		RateLimit: RateLimitConfig{RequestsPerMinute: 100},
		Email:     EmailConfig{MockLatency: 100 * time.Millisecond},
	}
}

func overrideFromEnv(cfg *Config) {
	setString(&cfg.App.Env, "APP_ENV")
	setString(&cfg.App.Address, "APP_ADDRESS")
	setString(&cfg.JWT.Secret, "JWT_SECRET")
	setDuration(&cfg.JWT.TTL, "JWT_TTL")
	setString(&cfg.MySQL.DSN, "MYSQL_DSN")
	setInt(&cfg.MySQL.MaxOpenConns, "MYSQL_MAX_OPEN_CONNS")
	setInt(&cfg.MySQL.MaxIdleConns, "MYSQL_MAX_IDLE_CONNS")
	setDuration(&cfg.MySQL.ConnMaxLifetime, "MYSQL_CONN_MAX_LIFETIME")
	setString(&cfg.Redis.Addr, "REDIS_ADDR")
	setString(&cfg.Redis.Password, "REDIS_PASSWORD")
	setInt(&cfg.Redis.DB, "REDIS_DB")
	setDuration(&cfg.Redis.TTL, "REDIS_TTL")
	setInt(&cfg.RateLimit.RequestsPerMinute, "RATE_LIMIT_REQUESTS_PER_MINUTE")
	setDuration(&cfg.Email.MockLatency, "EMAIL_MOCK_LATENCY")
}

func setString(target *string, key string) {
	if v := os.Getenv(key); v != "" {
		*target = v
	}
}

func setInt(target *int, key string) {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			*target = i
		}
	}
}

func setDuration(target *time.Duration, key string) {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			*target = d
		}
	}
}
