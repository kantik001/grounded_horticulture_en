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
const maxLLMHistoryMessages = 24

// ChatStore is persistent chat storage (PostgreSQL + files on disk).
type ChatStore struct {
	pool      *pgxpool.Pool
	uploadDir string
}

// Connects to Postgres and creates ChatStore with an upload directory.
func newChatStore(ctx context.Context, databaseURL, uploadDir string) (*ChatStore, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
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

// Closes the PostgreSQL connection pool.
func (st *ChatStore) Close() {
	if st != nil && st.pool != nil {
		st.pool.Close()
	}
}

// Applies one SQL migration file to the database and records it in
// schema_migrations within the same transaction.
func applyMigration(ctx context.Context, pool *pgxpool.Pool, sqlPath string) error {
	body, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", sqlPath, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, string(body)); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (filename) VALUES ($1)`,
		filepath.Base(sqlPath),
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}
	return tx.Commit(ctx)
}

// appliedMigrations returns the set of migration filenames already recorded
// in schema_migrations (creating the ledger table on first run).
func appliedMigrations(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}
	rows, err := pool.Query(ctx, `SELECT filename FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()
	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

// Applies pending .sql files from the migrations directory in name order,
// skipping ones already recorded in schema_migrations.
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
	applied, err := appliedMigrations(ctx, pool)
	if err != nil {
		return err
	}
	skipped := 0
	for _, f := range files {
		if applied[filepath.Base(f)] {
			skipped++
			continue
		}
		if err := applyMigration(ctx, pool, f); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		log.Printf("Applied migration: %s", filepath.Base(f))
	}
	if skipped > 0 {
		log.Printf("Migrations up to date: %d already applied", skipped)
	}
	return nil
}

// Finds the migrations directory (MIGRATIONS_DIR env or common paths).
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

// Generates a random chat session id (hex).
func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Generates a token for an uploaded image URL.
func newImageToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// UpsertUser creates or updates a user by telegram_id.
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

// SQL NULL for empty string, otherwise pointer to value.
func nullIfEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// CreateSession creates a new session for a user (users table id) and crop.
func (st *ChatStore) CreateSession(ctx context.Context, userID int64, cropID string) (string, error) {
	sid := newSessionID()
	_, err := st.pool.Exec(ctx,
		`INSERT INTO chat_sessions (id, user_id, crop_id) VALUES ($1, $2, $3)`,
		sid, userID, cropID,
	)
	return sid, err
}

// SessionCropID returns the session crop_id (with ownership check).
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

// sessionOwned checks that the session belongs to the telegram user.
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

// GetOrCreateSession returns an existing session or creates one with crop_id.
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

// scanChatMessage reads one messages row (with optional feedback).
func scanChatMessage(
	imageToken *string,
	classPred *string,
	classConf *float64,
	fbRating *int16,
	m *ChatMessage,
) {
	if imageToken != nil && *imageToken != "" {
		m.ImageURL = mediaURL(*imageToken)
	}
	if classPred != nil {
		m.ClassPrediction = *classPred
	}
	if classConf != nil {
		m.ClassConfidence = *classConf
	}
	if fbRating != nil {
		r := int(*fbRating)
		m.FeedbackRating = &r
	}
}

// finalizeStoredMessage sets the DB id and fills the media URL from the image token.
func finalizeStoredMessage(m ChatMessage, id int64) ChatMessage {
	m.ID = id
	if m.ImageToken != "" && m.ImageURL == "" {
		m.ImageURL = mediaURL(m.ImageToken)
	}
	return m
}

// ListMessages returns session history for the UI.
func (st *ChatStore) ListMessages(ctx context.Context, sessionID string, telegramID int64) ([]ChatMessage, error) {
	owned, err := st.sessionOwned(ctx, sessionID, telegramID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, errSessionNotFound
	}
	rows, err := st.pool.Query(ctx, `
		SELECT m.id, m.role, m.content, m.kind, m.image_token, m.class_prediction, m.class_confidence,
		       mf.rating
		FROM messages m
		LEFT JOIN users u ON u.telegram_id = $2
		LEFT JOIN message_feedback mf ON mf.message_id = m.id AND mf.user_id = u.id
		WHERE m.session_id = $1
		ORDER BY m.created_at ASC, m.id ASC`, sessionID, telegramID,
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
		var fbRating *int16
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.Kind, &imageToken, &classPred, &classConf, &fbRating); err != nil {
			return nil, err
		}
		scanChatMessage(imageToken, classPred, classConf, fbRating, &m)
		out = append(out, m)
	}
	return out, rows.Err()
}

// Public media file URL by token.
func mediaURL(token string) string {
	return "/api/media/" + token
}

var errSessionNotFound = fmt.Errorf("session not found")

// AppendMessage saves a message and trims history to maxSessionMessages.
func (st *ChatStore) AppendMessage(ctx context.Context, sessionID string, m ChatMessage) (ChatMessage, error) {
	var id int64
	err := st.pool.QueryRow(ctx, `
		INSERT INTO messages (session_id, role, content, kind, image_token, class_prediction, class_confidence)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		sessionID, m.Role, m.Content, m.Kind,
		nullToken(m.ImageToken), nullIfEmpty(m.ClassPrediction), nullConfidence(m.ClassConfidence),
	).Scan(&id)
	if err != nil {
		return ChatMessage{}, err
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
	return finalizeStoredMessage(m, id), err
}

// NULL for empty image_token on INSERT.
func nullToken(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// NULL for zero classification confidence.
func nullConfidence(v float64) *float64 {
	if v <= 0 {
		return nil
	}
	return &v
}

// HistoryForLLM returns recent session messages for the LLM (SQL LIMIT, not full history).
func (st *ChatStore) HistoryForLLM(ctx context.Context, sessionID string, telegramID int64, excludeLastN int) ([]Message, error) {
	if excludeLastN < 0 {
		excludeLastN = 0
	}
	owned, err := st.sessionOwned(ctx, sessionID, telegramID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, errSessionNotFound
	}

	limit := maxLLMHistoryMessages + excludeLastN
	rows, err := st.pool.Query(ctx, `
		SELECT m.id, m.role, m.content, m.kind, m.image_token, m.class_prediction, m.class_confidence
		FROM messages m
		JOIN chat_sessions cs ON cs.id = m.session_id
		JOIN users u ON u.id = cs.user_id
		WHERE m.session_id = $1 AND u.telegram_id = $2
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT $3`, sessionID, telegramID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []ChatMessage
	for rows.Next() {
		var m ChatMessage
		var imageToken *string
		var classPred *string
		var classConf *float64
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.Kind, &imageToken, &classPred, &classConf); err != nil {
			return nil, err
		}
		scanChatMessage(imageToken, classPred, classConf, nil, &m)
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	if excludeLastN > 0 && len(msgs) > excludeLastN {
		msgs = msgs[:len(msgs)-excludeLastN]
	}

	var out []Message
	for _, m := range msgs {
		if msg, ok := m.toLLMMessage(); ok {
			out = append(out, msg)
		}
	}
	return trimHistoryMessages(out, maxLLMHistoryMessages), nil
}

// SaveImage stores JPEG/PNG on disk and returns a URL token.
func (st *ChatStore) SaveImage(data []byte) (string, error) {
	token := newImageToken()
	path := filepath.Join(st.uploadDir, token+".bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return token, nil
}

// UserCanAccessImage checks that the file belongs to the user's message.
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

// ReadImage returns file bytes by token.
func (st *ChatStore) ReadImage(token string) ([]byte, error) {
	token = strings.TrimSpace(token)
	if token == "" || strings.Contains(token, "..") || strings.Contains(token, "/") {
		return nil, fmt.Errorf("invalid token")
	}
	return os.ReadFile(filepath.Join(st.uploadDir, token+".bin"))
}

// Waits for Postgres readiness at startup (docker compose).
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
