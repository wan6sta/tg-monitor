package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	gotdtg "github.com/gotd/td/tg"

	"github.com/wan6sta/tg-monitor/internal/config"
	"github.com/wan6sta/tg-monitor/internal/repo"
	"github.com/wan6sta/tg-monitor/internal/services"
	"github.com/wan6sta/tg-monitor/internal/utils"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	cfgPath := "configs/config.yml"
	if len(args) > 0 {
		cfgPath = args[0]
	}

	switch command {
	case "run":
		if err := cmdRun(log, cfgPath); err != nil {
			log.Error("ошибка", "err", err)
			os.Exit(1)
		}
	case "export":
		if err := cmdExport(log, cfgPath); err != nil {
			log.Error("ошибка", "err", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "неизвестная команда: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`tg-monitor — мониторинг Telegram чатов по ключевым словам

Использование:
  monitor run    [config.yml]   — запустить мониторинг (Ctrl+C для остановки)
  monitor export [config.yml]   — экспортировать результаты в xlsx

Конфиг по умолчанию: configs/config.yml`)
}

func cmdRun(log *slog.Logger, cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("конфиг: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repository, err := repo.New(ctx, cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("база данных: %w", err)
	}
	defer repository.Close()

	if err := repository.Migrate(ctx); err != nil {
		return fmt.Errorf("миграция: %w", err)
	}
	log.Info("база данных готова", "path", cfg.Database.Path)

	_ = os.MkdirAll(".session", 0700)
	client := telegram.NewClient(cfg.Telegram.AppID, cfg.Telegram.AppHash, telegram.Options{
		SessionStorage: &session.FileStorage{
			Path: filepath.Join(".session", "session.json"),
		},
	})

	mon := services.NewMonitor(cfg.Keywords, repository, log)

	err = client.Run(ctx, func(ctx context.Context) error {
		flow := auth.NewFlow(
			// TODO: поменять пароль из конфига
			auth.Constant(cfg.Telegram.Phone, cfg.Telegram.Password, auth.CodeAuthenticatorFunc(
				func(ctx context.Context, sentCode *gotdtg.AuthSentCode) (string, error) {
					fmt.Printf("Тип кода: %T\n", sentCode.Type)
					fmt.Printf("Телефон: %s\n", cfg.Telegram.Phone)
					utils.PrintPassword(cfg.Telegram.Password)
					fmt.Print("Введите код из Telegram: ")
					var code string
					fmt.Scanln(&code)
					return strings.TrimSpace(code), nil
				},
			)),
			auth.SendCodeOptions{
				CurrentNumber: true,
			},
		)
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return fmt.Errorf("авторизация: %w", err)
		}

		self, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("self: %w", err)
		}
		log.Info("авторизован", "пользователь", self.Username, "id", self.ID)

		api := client.API()

		chats, err := services.ResolveAll(ctx, api, cfg.Sources.Forums, cfg.Sources.Chats, log)
		if err != nil {
			return fmt.Errorf("resolve chats: %w", err)
		}

		for _, ch := range chats {
			topics, err := services.GetForumTopics(ctx, api, ch)
			if err != nil {
				log.Warn("не удалось получить топики", "chat", ch.Title, "err", err)
			}
			mon.RegisterChat(ch, topics)
			if len(topics) > 0 {
				log.Info("топики форума загружены", "chat", ch.Title, "count", len(topics))
			}
		}

		return mon.Listen(ctx, client)
	})

	if err != nil && err != context.Canceled {
		return err
	}
	log.Info("остановлено")
	return nil
}

func cmdExport(log *slog.Logger, cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("конфиг: %w", err)
	}

	ctx := context.Background()

	repository, err := repo.New(ctx, cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("база данных: %w", err)
	}
	defer repository.Close()

	messages, err := repository.All(ctx)
	if err != nil {
		return fmt.Errorf("чтение сообщений: %w", err)
	}

	if len(messages) == 0 {
		log.Info("нет сообщений для экспорта")
		return nil
	}

	outDir := cfg.Export.Dir
	if outDir == "" {
		outDir = "."
	}
	_ = os.MkdirAll(outDir, 0755)

	filename := filepath.Join(outDir, fmt.Sprintf("export_%s.xlsx", time.Now().Format("2006-01-02_15-04-05")))
	if err := services.ToXLSX(messages, filename); err != nil {
		return fmt.Errorf("экспорт: %w", err)
	}

	log.Info("экспорт завершён", "файл", filename, "сообщений", len(messages))
	return nil
}
