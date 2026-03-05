package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Message struct {
	ChatID     int64
	ChatTitle  string
	TopicID    int
	TopicTitle string
	MessageID  int
	SenderID   int64
	SenderName string
	Text       string
	Keyword    string
	SentAt     time.Time
}

type Repository struct {
	db *sql.DB
}

func New(ctx context.Context, path string) (*Repository, error) {
	db, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite не поддерживает параллельную запись
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Close() {
	r.db.Close()
}

func (r *Repository) Migrate(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS messages (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id     INTEGER NOT NULL,
			chat_title  TEXT    NOT NULL,
			topic_id    INTEGER NOT NULL DEFAULT 0,
			topic_title TEXT    NOT NULL DEFAULT '',
			message_id  INTEGER NOT NULL,
			sender_id   INTEGER,
			sender_name TEXT,
			text        TEXT    NOT NULL,
			keyword     TEXT    NOT NULL,
			sent_at     DATETIME NOT NULL,
			saved_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			UNIQUE (chat_id, message_id)
		);
		CREATE INDEX IF NOT EXISTS idx_messages_keyword ON messages (keyword);
		CREATE INDEX IF NOT EXISTS idx_messages_sent_at ON messages (sent_at DESC);
	`)
	return err
}

// Save сохраняет сообщение. При дубле — игнорирует.
func (r *Repository) Save(ctx context.Context, m Message) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO messages
			(chat_id, chat_title, topic_id, topic_title, message_id, sender_id, sender_name, text, keyword, sent_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)
	`, m.ChatID, m.ChatTitle, m.TopicID, m.TopicTitle,
		m.MessageID, m.SenderID, m.SenderName, m.Text, m.Keyword, m.SentAt)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

// All возвращает все сообщения для экспорта.
func (r *Repository) All(ctx context.Context) ([]Message, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT chat_id, chat_title, topic_id, topic_title,
		       message_id, sender_id, sender_name, text, keyword, sent_at
		FROM messages
		ORDER BY sent_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		var sentAt string
		if err := rows.Scan(
			&m.ChatID, &m.ChatTitle, &m.TopicID, &m.TopicTitle,
			&m.MessageID, &m.SenderID, &m.SenderName, &m.Text, &m.Keyword, &sentAt,
		); err != nil {
			return nil, err
		}
		m.SentAt, _ = time.Parse("2006-01-02T15:04:05Z", sentAt)
		if m.SentAt.IsZero() {
			m.SentAt, _ = time.Parse("2006-01-02 15:04:05", sentAt)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
