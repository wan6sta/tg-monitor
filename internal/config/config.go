package config

import (
	"fmt"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Database DatabaseConfig `yaml:"database"`
	Keywords []string       `yaml:"keywords"  env-required:"true"`
	Sources  SourcesConfig  `yaml:"sources"`
	Export   ExportConfig   `yaml:"export"`
}

type TelegramConfig struct {
	AppID    int    `yaml:"app_id"   env:"TG_APP_ID"   env-required:"true"`
	AppHash  string `yaml:"app_hash" env:"TG_APP_HASH" env-required:"true"`
	Phone    string `yaml:"phone"    env:"TG_PHONE"    env-required:"true"`
	Password string `yaml:"password" env:"TG_PASSWORD" env-required:"false"`
}

type DatabaseConfig struct {
	// Path — путь к файлу SQLite. По умолчанию monitor.db рядом с бинарником.
	Path string `yaml:"path" env:"DB_PATH" env-default:"monitor.db"`
}

type SourcesConfig struct {
	Forums []string `yaml:"forums"`
	Chats  []string `yaml:"chats"`
}

type ExportConfig struct {
	Dir string `yaml:"dir" env:"EXPORT_DIR" env-default:"."`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, err
	}

	if len(cfg.Sources.Forums) == 0 && len(cfg.Sources.Chats) == 0 {
		return nil, fmt.Errorf("не указан ни один источник (sources.forums или sources.chats)")
	}

	cfg.normalize()
	return &cfg, nil
}

func (c *Config) normalize() {
	for i, kw := range c.Keywords {
		c.Keywords[i] = strings.ToLower(strings.TrimSpace(kw))
	}
}
