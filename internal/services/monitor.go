package services

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	gotdtg "github.com/gotd/td/tg"

	"github.com/wan6sta/tg-monitor/internal/repo"
)

type Monitor struct {
	keywords []string
	repo     *repo.Repository
	log      *slog.Logger

	// chatID → chat title, для быстрого lookup
	chatTitles map[int64]string
	// chatID → map[topicID]topicTitle
	topicTitles map[int64]map[int]string
}

func NewMonitor(keywords []string, repo *repo.Repository, log *slog.Logger) *Monitor {
	return &Monitor{
		keywords:    keywords,
		repo:        repo,
		log:         log,
		chatTitles:  make(map[int64]string),
		topicTitles: make(map[int64]map[int]string),
	}
}

// RegisterChat регистрирует чат для быстрого lookup заголовков.
func (m *Monitor) RegisterChat(chat Chat, topics []ForumTopic) {
	m.chatTitles[chat.ID] = chat.Title
	if len(topics) > 0 {
		tm := make(map[int]string, len(topics))
		for _, t := range topics {
			tm[t.ID] = t.Title
		}
		m.topicTitles[chat.ID] = tm
	}
}

// Listen запускает приём апдейтов. Блокирует до отмены ctx.
func (m *Monitor) Listen(ctx context.Context, client *telegram.Client) error {
	dispatcher := gotdtg.NewUpdateDispatcher()
	dispatcher.OnNewMessage(m.handleNewMessage())
	dispatcher.OnNewChannelMessage(m.handleNewChannelMessage())

	gaps := updates.New(updates.Config{
		Handler: dispatcher,
	})

	return gaps.Run(ctx, client.API(), 0, updates.AuthOptions{
		OnStart: func(ctx context.Context) {
			m.log.Info("слушаю новые сообщения, Ctrl+C для остановки")
		},
	})
}

func (m *Monitor) handleNewMessage() gotdtg.NewMessageHandler {
	return func(ctx context.Context, e gotdtg.Entities, upd *gotdtg.UpdateNewMessage) error {
		msg, ok := upd.Message.(*gotdtg.Message)
		if !ok || msg.Out {
			return nil
		}
		return m.process(ctx, e, msg)
	}
}

func (m *Monitor) handleNewChannelMessage() gotdtg.NewChannelMessageHandler {
	return func(ctx context.Context, e gotdtg.Entities, upd *gotdtg.UpdateNewChannelMessage) error {
		msg, ok := upd.Message.(*gotdtg.Message)
		if !ok || msg.Out {
			return nil
		}
		return m.process(ctx, e, msg)
	}
}

func (m *Monitor) process(ctx context.Context, e gotdtg.Entities, msg *gotdtg.Message) error {
	text := msg.Message
	if text == "" {
		return nil
	}

	keyword := m.matchKeyword(text)
	if keyword == "" {
		return nil
	}

	chatID, chatTitle := m.resolvePeer(e, msg.PeerID)
	senderID, senderName := m.resolveSender(e, msg.FromID)

	// Топик форума
	topicID := 0
	topicTitle := ""
	if msg.ReplyTo != nil {
		if rt, ok := msg.ReplyTo.(*gotdtg.MessageReplyHeader); ok && rt.ForumTopic {
			if topID, ok := rt.GetReplyToTopID(); ok {
				topicID = topID
			} else {
				// первое сообщение топика — его ID и есть ID топика
				if msgID, ok := rt.GetReplyToMsgID(); ok {
					topicID = msgID
				}
			}
			if tm, ok := m.topicTitles[chatID]; ok {
				topicTitle = tm[topicID]
			}
		}
	}

	m.log.Info("совпадение",
		"чат", chatTitle,
		"топик", topicTitle,
		"ключевое_слово", keyword,
		"отправитель", senderName,
		"текст", truncate(text, 80),
	)

	return m.repo.Save(ctx, repo.Message{
		ChatID:     chatID,
		ChatTitle:  chatTitle,
		TopicID:    topicID,
		TopicTitle: topicTitle,
		MessageID:  msg.ID,
		SenderID:   senderID,
		SenderName: senderName,
		Text:       text,
		Keyword:    keyword,
		SentAt:     time.Unix(int64(msg.Date), 0),
	})
}

func (m *Monitor) matchKeyword(text string) string {
	lower := strings.ToLower(text)
	for _, kw := range m.keywords {
		if containsWord(lower, kw) {
			return kw
		}
	}
	return ""
}

func containsWord(text, kw string) bool {
	idx := 0
	for {
		pos := strings.Index(text[idx:], kw)
		if pos == -1 {
			return false
		}
		pos += idx
		before := pos == 0 || !isLetter(rune(text[pos-1]))
		after := pos+len(kw) >= len(text) || !isLetter(rune(text[pos+len(kw)]))
		if before && after {
			return true
		}
		idx = pos + 1
	}
}

func isLetter(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (m *Monitor) resolvePeer(e gotdtg.Entities, peer gotdtg.PeerClass) (int64, string) {
	switch p := peer.(type) {
	case *gotdtg.PeerChannel:
		if title, ok := m.chatTitles[p.ChannelID]; ok {
			return p.ChannelID, title
		}
		if ch, ok := e.Channels[p.ChannelID]; ok {
			return p.ChannelID, ch.Title
		}
		return p.ChannelID, "channel"
	case *gotdtg.PeerChat:
		if title, ok := m.chatTitles[p.ChatID]; ok {
			return p.ChatID, title
		}
		if ch, ok := e.Chats[p.ChatID]; ok {
			return p.ChatID, ch.Title
		}
		return p.ChatID, "chat"
	case *gotdtg.PeerUser:
		return p.UserID, "личка"
	}
	return 0, "unknown"
}

func (m *Monitor) resolveSender(e gotdtg.Entities, from gotdtg.PeerClass) (int64, string) {
	if from == nil {
		return 0, "аноним"
	}
	if p, ok := from.(*gotdtg.PeerUser); ok {
		if u, ok := e.Users[p.UserID]; ok {
			name := strings.TrimSpace(u.FirstName + " " + u.LastName)
			if name == "" {
				name = u.Username
			}
			return p.UserID, name
		}
		return p.UserID, "user"
	}
	return 0, "аноним"
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
