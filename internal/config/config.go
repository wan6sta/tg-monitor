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
	AppID   int    `yaml:"app_id"   env:"TG_APP_ID"   env-required:"true"`
	AppHash string `yaml:"app_hash" env:"TG_APP_HASH" env-required:"true"`
	Phone   string `yaml:"phone"    env:"TG_PHONE"    env-required:"true"`
}

type DatabaseConfig struct {
	URL string `yaml:"url" env:"DATABASE_URL" env-required:"true"`
}

type SourcesConfig struct {
	// Forums — форум-группы с топиками. Скрипт сам обойдёт все вложенные темы.
	Forums []string `yaml:"forums"`
	// Chats — обычные каналы/группы.
	Chats []string `yaml:"chats"`
}

type ExportConfig struct {
	// Dir — папка куда сохранять xlsx. По умолчанию текущая директория.
	Dir string `yaml:"dir" env:"EXPORT_DIR" env-default:"."`
}

// Load читает конфиг из yml-файла.
// Все поля с тегом env можно также переопределить через переменные окружения —
// это удобно в CI или если не хочется хранить credentials в файле.
func Load(path string) (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("не удалось прочитать конфиг: %w", err)
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
