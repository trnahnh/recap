package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const LoopbackHost = "127.0.0.1"

const (
	DefaultPort       = 55432
	DefaultPoolSize   = 10
	DefaultMaxRecords = 5
	DefaultMaxTokens  = 2000
	DefaultDBName     = "recap"
	DefaultDBUser     = "recap"
)

type Config struct {
	DB      DBConfig      `json:"db"`
	Context ContextConfig `json:"context"`
}

type DBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Name     string `json:"name"`
	PoolSize int    `json:"pool_size"`
}

type ContextConfig struct {
	MaxRecords int `json:"max_records"`
	MaxTokens  int `json:"max_tokens"`
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating user config dir: %w", err)
	}
	return filepath.Join(dir, "recap", "config.json"), nil
}

func Generate() (*Config, error) {
	pw, err := randomSecret(24)
	if err != nil {
		return nil, err
	}
	return &Config{
		DB: DBConfig{
			Host:     LoopbackHost,
			Port:     DefaultPort,
			User:     DefaultDBUser,
			Password: pw,
			Name:     DefaultDBName,
			PoolSize: DefaultPoolSize,
		},
		Context: ContextConfig{
			MaxRecords: DefaultMaxRecords,
			MaxTokens:  DefaultMaxTokens,
		},
	}, nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &c, nil
}

func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("securing config %s: %w", path, err)
	}
	return nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.Name,
	)
}

func (c *Config) ComposeEnv() []string {
	return []string{
		"RECAP_DB_USER=" + c.DB.User,
		"RECAP_DB_PASSWORD=" + c.DB.Password,
		"RECAP_DB_NAME=" + c.DB.Name,
		fmt.Sprintf("RECAP_DB_PORT=%d", c.DB.Port),
	}
}

func randomSecret(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating credential: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
