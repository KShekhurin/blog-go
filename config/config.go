package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv      string
	ServerPort  string
	DatabaseURL string
	JWTSecret   string

	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
	SignMethod jwt.SigningMethod

	HashParams *argon2id.Params
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func fromStringToPrivateKey(keyString string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(keyString))

	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)

	if err != nil {
		return nil, err
	}

	pk, ok := key.(ed25519.PrivateKey)

	if !ok {
		return nil, fmt.Errorf("expected Ed25519 private key, got %T", key)
	}

	return pk, nil
}

func Load() (*Config, error) {
	// Load .env file (only in development)
	_ = godotenv.Load() // ignores error if .env doesn't exist

	cfg := &Config{}

	// You can use manual binding or use a library like envconfig / viper
	cfg.AppEnv = getEnv("APP_ENV", "development")
	cfg.ServerPort = getEnv("SERVER_PORT", "8080")
	cfg.DatabaseURL = getEnv("DATABASE_URL", "")
	cfg.JWTSecret = getEnv("JWT_SECRET", "")
	argon2Memory, err := strconv.ParseUint(getEnv("ARGON2_MEMORY", "65536"), 10, 32)

	if err != nil {
		return nil, fmt.Errorf("failed to parse ARGON2_MEMORY: %s", err)
	}

	private, err := fromStringToPrivateKey(cfg.JWTSecret)

	if err != nil {
		return nil, err
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	cfg.PrivateKey = private
	cfg.PublicKey = private.Public().(ed25519.PublicKey)
	cfg.SignMethod = jwt.SigningMethodEdDSA

	cfg.HashParams = &argon2id.Params{
		Memory:      uint32(argon2Memory),
		Iterations:  4,
		Parallelism: min(uint8(runtime.NumCPU()), 4),
		SaltLength:  16,
		KeyLength:   32,
	}

	return cfg, nil
}
