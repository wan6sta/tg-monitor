package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	gotdtg "github.com/gotd/td/tg"
)

type Chat struct {
	ID    int64
	Title string
	Input *gotdtg.InputChannel
}

type ForumTopic struct {
	ID    int
	Title string
}

func ResolveAll(ctx context.Context, api *gotdtg.Client, forums, chats []string, log *slog.Logger) ([]Chat, error) {
	var result []Chat

	for _, f := range forums {
		log.Info("разворачиваю форум", "url", f)
		ch, err := joinAndGet(ctx, api, f, log)
		if err != nil {
			log.Warn("не удалось получить форум", "url", f, "err", err)
			continue
		}
		result = append(result, *ch)
		log.Info("форум добавлен", "title", ch.Title, "id", ch.ID)
	}

	for _, c := range chats {
		ch, err := joinAndGet(ctx, api, c, log)
		if err != nil {
			log.Warn("не удалось вступить", "chat", c, "err", err)
			continue
		}
		result = append(result, *ch)
		log.Info("чат добавлен", "title", ch.Title, "id", ch.ID)
	}

	return result, nil
}

func joinAndGet(ctx context.Context, api *gotdtg.Client, target string, log *slog.Logger) (*Chat, error) {
	target = strings.TrimSpace(target)
	if isInviteLink(target) {
		return joinByInvite(ctx, api, target, log)
	}
	return joinByUsername(ctx, api, extractUsername(target), log)
}

func joinByUsername(ctx context.Context, api *gotdtg.Client, username string, log *slog.Logger) (*Chat, error) {
	resolved, err := api.ContactsResolveUsername(ctx, &gotdtg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve @%s: %w", username, err)
	}
	if len(resolved.Chats) == 0 {
		return nil, fmt.Errorf("чат @%s не найден", username)
	}

	switch ch := resolved.Chats[0].(type) {
	case *gotdtg.Channel:
		input := ch.AsInput() // *gotdtg.InputChannel
		_, err = api.ChannelsJoinChannel(ctx, input)
		if err != nil && !isAlreadyParticipant(err) {
			log.Warn("ошибка при вступлении", "username", username, "err", err)
		}
		time.Sleep(600 * time.Millisecond)
		return &Chat{ID: ch.ID, Title: ch.Title, Input: input}, nil

	case *gotdtg.Chat:
		return &Chat{ID: ch.ID, Title: ch.Title}, nil
	}

	return nil, fmt.Errorf("неизвестный тип чата для @%s", username)
}

func joinByInvite(ctx context.Context, api *gotdtg.Client, link string, log *slog.Logger) (*Chat, error) {
	hash := extractInviteHash(link)
	result, err := api.MessagesImportChatInvite(ctx, hash)
	if err != nil {
		if !isAlreadyParticipant(err) {
			return nil, fmt.Errorf("invite join %s: %w", link, err)
		}
		info, err2 := api.MessagesCheckChatInvite(ctx, hash)
		if err2 != nil {
			return nil, fmt.Errorf("check invite %s: %w", link, err2)
		}
		if already, ok := info.(*gotdtg.ChatInviteAlready); ok {
			switch ch := already.Chat.(type) {
			case *gotdtg.Channel:
				return &Chat{ID: ch.ID, Title: ch.Title, Input: ch.AsInput()}, nil
			case *gotdtg.Chat:
				return &Chat{ID: ch.ID, Title: ch.Title}, nil
			}
		}
		return nil, fmt.Errorf("уже участник, но не удалось получить данные чата")
	}

	time.Sleep(600 * time.Millisecond)

	updates, ok := result.(*gotdtg.Updates)
	if !ok || len(updates.Chats) == 0 {
		return nil, fmt.Errorf("неожиданный ответ на invite join")
	}
	switch ch := updates.Chats[0].(type) {
	case *gotdtg.Channel:
		return &Chat{ID: ch.ID, Title: ch.Title, Input: ch.AsInput()}, nil
	case *gotdtg.Chat:
		return &Chat{ID: ch.ID, Title: ch.Title}, nil
	}
	return nil, fmt.Errorf("неизвестный тип чата в ответе")
}

// GetForumTopics возвращает все топики форум-группы.
func GetForumTopics(ctx context.Context, api *gotdtg.Client, ch Chat) ([]ForumTopic, error) {
	if ch.Input == nil {
		return nil, nil
	}

	var topics []ForumTopic
	offsetID := 0

	for {
		res, err := api.MessagesGetForumTopics(ctx, &gotdtg.MessagesGetForumTopicsRequest{
			Peer: &gotdtg.InputPeerChannel{
				ChannelID:  ch.Input.ChannelID,
				AccessHash: ch.Input.AccessHash,
			},
			OffsetTopic: offsetID,
			Limit:       100,
		})
		if err != nil {
			return nil, fmt.Errorf("GetForumTopics: %w", err)
		}

		for _, t := range res.Topics {
			if topic, ok := t.(*gotdtg.ForumTopic); ok {
				topics = append(topics, ForumTopic{
					ID:    topic.ID,
					Title: topic.Title,
				})
			}
		}

		if len(res.Topics) < 100 {
			break
		}
		offsetID = topics[len(topics)-1].ID
	}

	return topics, nil
}

func isInviteLink(s string) bool {
	return strings.Contains(s, "/+") || strings.HasPrefix(s, "+")
}

func extractUsername(s string) string {
	s = strings.TrimPrefix(s, "https://t.me/")
	s = strings.TrimPrefix(s, "t.me/")
	s = strings.TrimPrefix(s, "@")
	if idx := strings.Index(s, "/"); idx != -1 {
		s = s[:idx]
	}
	return s
}

func extractInviteHash(link string) string {
	parts := strings.Split(link, "/+")
	if len(parts) == 2 {
		return parts[1]
	}
	return strings.TrimPrefix(link, "+")
}

func isAlreadyParticipant(err error) bool {
	return err != nil && strings.Contains(err.Error(), "ALREADY_PARTICIPANT")
}
