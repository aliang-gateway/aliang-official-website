package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Register RegisterConfig `yaml:"register"`
	SMTP     SMTPConfig     `yaml:"smtp"`
	Redis    RedisConfig    `yaml:"redis"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type AuthConfig struct {
	AdminBootstrapSecret string `yaml:"admin_bootstrap_secret"`
}

type RegisterConfig struct {
	AllowedEmailDomains      []string `yaml:"allowed_email_domains"`
	RequireEmailVerification *bool    `yaml:"require_email_verification"`
}

type SMTPConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	FromName string `yaml:"from_name"`
}

type RedisConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

func Load(path string) (*Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("config path is required")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml config: %w", err)
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if strings.TrimSpace(c.Server.Port) == "" {
		c.Server.Port = "8080"
	}
	if strings.TrimSpace(c.Database.Path) == "" {
		c.Database.Path = "./data.db"
	}
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.Server.Port) == "" {
		return fmt.Errorf("server.port is required")
	}
	if strings.TrimSpace(c.Database.Path) == "" {
		return fmt.Errorf("database.path is required")
	}

	if c.SMTP.Enabled {
		if strings.TrimSpace(c.SMTP.Host) == "" || c.SMTP.Port <= 0 || strings.TrimSpace(c.SMTP.Username) == "" || strings.TrimSpace(c.SMTP.Password) == "" || strings.TrimSpace(c.SMTP.From) == "" {
			return fmt.Errorf("smtp enabled requires host, port, username, password, from")
		}
	}

	return nil
}
