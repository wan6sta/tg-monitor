package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"tg-monitor/internal/config"
	"tg-monitor/internal/db"
	tgclient "tg-monitor/internal/tg"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfgPath := "config.yml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Error("ошибка конфига", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo, err := db.New(ctx, cfg.Database.URL)
	if err != nil {
		log.Error("ошибка подключения к БД", "err", err)
		os.Exit(1)
	}
	defer repo.Close()

	if err := repo.Migrate(ctx); err != nil {
		log.Error("ошибка миграции", "err", err)
		os.Exit(1)
	}

	client := tgclient.NewClient(cfg.Telegram)
	mon := tgclient.NewMonitor(cfg.Keywords, repo, log)

	err = client.Run(ctx, func(ctx context.Context) error {
		if err := tgclient.Authorize(ctx, client, cfg.Telegram.Phone); err != nil {
			return fmt.Errorf("авторизация: %w", err)
		}

		self, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("self: %w", err)
		}
		log.Info("авторизован", "пользователь", self.Username, "id", self.ID)

		api := client.API()

		chats, err := tgclient.ResolveAll(ctx, api, cfg.Sources.Forums, cfg.Sources.Chats, log)
		if err != nil {
			return fmt.Errorf("resolve chats: %w", err)
		}

		for _, ch := range chats {
			topics, err := tgclient.GetForumTopics(ctx, api, ch)
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
		log.Error("ошибка", "err", err)
		os.Exit(1)
	}
	log.Info("остановлено")
}
