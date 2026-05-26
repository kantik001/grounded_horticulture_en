package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const maxSessionMessages = 80

// ChatStore — персистентное хранилище чата (PostgreSQL + файлы на диске).
type ChatStore struct {
	pool      *pgxpool.Pool
	uploadDir string
}

func newChatStore(ctx context.Context, databaseURL, uploadDir string) (*ChatStore, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("DATABASE_URL не задан")
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("upload dir: %w", err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	return &ChatStore{pool: pool, uploadDir: uploadDir}, nil
}

func (st *ChatStore) Close() {
	if st != nil && st.pool != nil {
		st.pool.Close()
	}
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool, sqlPath string) error {
	body, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", sqlPath, err)
	}
	_, err = pool.Exec(ctx, string(body))
	if err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}
	return nil
}

func runAllMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".sql") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no .sql migrations in %s", dir)
	}
	for _, f := range files {
		if err := runMigrations(ctx, pool, f); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		log.Printf("Applied migration: %s", filepath.Base(f))
	}
	return nil
}

func findMigrationsDir() (string, error) {
	if p := os.Getenv("MIGRATIONS_DIR"); p != "" {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p, nil
		}
	}
	for _, candidate := range []string{
		"/migrations",
		filepath.Join("..", "migrations"),
		filepath.Join("migrations"),
	} {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("migrations directory not found")
}

func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func newImageToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// UpsertUser создаёт или обновляет пользователя по telegram_id.
func (st *ChatStore) UpsertUser(ctx context.Context, u *TelegramUser) (int64, error) {
	var id int64
	err := st.pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, username, first_name, last_name, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (telegram_id) DO UPDATE SET
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			updated_at = NOW()
		RETURNING id`,
		u.ID, nullIfEmpty(u.Username), nullIfEmpty(u.FirstName), nullIfEmpty(u.LastName),
	).Scan(&id)
	return id, err
}

func nullIfEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// CreateSession создаёт новую сессию для пользователя (internal user id) и культуры.
func (st *ChatStore) CreateSession(ctx context.Context, userID int64, cropID string) (string, error) {
	sid := newSessionID()
	_, err := st.pool.Exec(ctx,
		`INSERT INTO chat_sessions (id, user_id, crop_id) VALUES ($1, $2, $3)`,
		sid, userID, cropID,
	)
	return sid, err
}

// SessionCropID возвращает crop_id сессии (с проверкой владельца).
func (st *ChatStore) SessionCropID(ctx context.Context, sessionID string, telegramID int64) (string, error) {
	var cropID string
	err := st.pool.QueryRow(ctx, `
		SELECT cs.crop_id FROM chat_sessions cs
		JOIN users u ON u.id = cs.user_id
		WHERE cs.id = $1 AND u.telegram_id = $2`, sessionID, telegramID,
	).Scan(&cropID)
	if err != nil {
		return "", errSessionNotFound
	}
	return cropID, nil
}

// sessionOwned проверяет, что сессия принадлежит telegram-пользователю.
func (st *ChatStore) sessionOwned(ctx context.Context, sessionID string, telegramID int64) (bool, error) {
	var ok bool
	err := st.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM chat_sessions cs
			JOIN users u ON u.id = cs.user_id
			WHERE cs.id = $1 AND u.telegram_id = $2
		)`, sessionID, telegramID,
	).Scan(&ok)
	return ok, err
}

// GetOrCreateSession возвращает существующую сессию или создаёт новую с crop_id.
func (st *ChatStore) GetOrCreateSession(ctx context.Context, sessionID string, u *TelegramUser, cropID string) (string, string, error) {
	userID, err := st.UpsertUser(ctx, u)
	if err != nil {
		return "", "", err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID != "" {
		owned, err := st.sessionOwned(ctx, sessionID, u.ID)
		if err != nil {
			return "", "", err
		}
		if owned {
			crop, err := st.SessionCropID(ctx, sessionID, u.ID)
			if err != nil {
				return "", "", err
			}
			return sessionID, crop, nil
		}
	}
	sid, err := st.CreateSession(ctx, userID, cropID)
	return sid, cropID, err
}

// ListMessages возвращает историю сессии для UI.
func (st *ChatStore) ListMessages(ctx context.Context, sessionID string, telegramID int64) ([]ChatMessage, error) {
	owned, err := st.sessionOwned(ctx, sessionID, telegramID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, errSessionNotFound
	}
	rows, err := st.pool.Query(ctx, `
		SELECT role, content, kind, image_token, class_prediction, class_confidence
		FROM messages
		WHERE session_id = $1
		ORDER BY created_at ASC, id ASC`, sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ChatMessage
	for rows.Next() {
		var m ChatMessage
		var imageToken *string
		var classPred *string
		var classConf *float64
		if err := rows.Scan(&m.Role, &m.Content, &m.Kind, &imageToken, &classPred, &classConf); err != nil {
			return nil, err
		}
		if imageToken != nil && *imageToken != "" {
			m.ImageURL = mediaURL(*imageToken)
		}
		if classPred != nil {
			m.ClassPrediction = *classPred
		}
		if classConf != nil {
			m.ClassConfidence = *classConf
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func mediaURL(token string) string {
	return "/api/media/" + token
}

var errSessionNotFound = fmt.Errorf("session not found")

// AppendMessage сохраняет сообщение и обрезает историю до maxSessionMessages.
func (st *ChatStore) AppendMessage(ctx context.Context, sessionID string, m ChatMessage) error {
	_, err := st.pool.Exec(ctx, `
		INSERT INTO messages (session_id, role, content, kind, image_token, class_prediction, class_confidence)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		sessionID, m.Role, m.Content, m.Kind,
		nullToken(m.ImageToken), nullIfEmpty(m.ClassPrediction), nullConfidence(m.ClassConfidence),
	)
	if err != nil {
		return err
	}
	_, err = st.pool.Exec(ctx, `
		DELETE FROM messages
		WHERE session_id = $1
		  AND id NOT IN (
			SELECT id FROM messages
			WHERE session_id = $1
			ORDER BY created_at DESC, id DESC
			LIMIT $2
		  )`, sessionID, maxSessionMessages,
	)
	return err
}

func nullToken(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func nullConfidence(v float64) *float64 {
	if v <= 0 {
		return nil
	}
	return &v
}

// HistoryForLLM — последние сообщения сессии в формате LLM.
func (st *ChatStore) HistoryForLLM(ctx context.Context, sessionID string, telegramID int64, excludeLastN int) ([]Message, error) {
	msgs, err := st.ListMessages(ctx, sessionID, telegramID)
	if err != nil {
		return nil, err
	}
	n := len(msgs) - excludeLastN
	if n < 0 {
		n = 0
	}
	var out []Message
	for _, m := range msgs[:n] {
		if msg, ok := m.toLLMMessage(); ok {
			out = append(out, msg)
		}
	}
	return trimHistoryMessages(out, 24), nil
}

// SaveImage сохраняет JPEG/PNG на диск, возвращает token для URL.
func (st *ChatStore) SaveImage(data []byte) (string, error) {
	token := newImageToken()
	path := filepath.Join(st.uploadDir, token+".bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return token, nil
}

// UserCanAccessImage проверяет, что файл принадлежит сообщению пользователя.
func (st *ChatStore) UserCanAccessImage(ctx context.Context, token string, telegramID int64) (bool, error) {
	var ok bool
	err := st.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM messages m
			JOIN chat_sessions cs ON cs.id = m.session_id
			JOIN users u ON u.id = cs.user_id
			WHERE m.image_token = $1 AND u.telegram_id = $2
		)`, token, telegramID,
	).Scan(&ok)
	return ok, err
}

// ReadImage возвращает байты файла по token.
func (st *ChatStore) ReadImage(token string) ([]byte, error) {
	token = strings.TrimSpace(token)
	if token == "" || strings.Contains(token, "..") || strings.Contains(token, "/") {
		return nil, fmt.Errorf("invalid token")
	}
	return os.ReadFile(filepath.Join(st.uploadDir, token+".bin"))
}

// WaitForPostgres retries ping (docker compose startup).
func waitForPostgres(ctx context.Context, databaseURL string, attempts int) (*pgxpool.Pool, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		pool, err := pgxpool.New(ctx, databaseURL)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, fmt.Errorf("postgres not ready after %d attempts: %v", attempts, lastErr)
}
