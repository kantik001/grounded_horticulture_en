package main

import (
	"context"
	"time"
)

// FeedbackReportItem — одна оценка ответа с контекстом диалога.
type FeedbackReportItem struct {
	FeedbackAt  time.Time `json:"feedback_at"`
	Rating      int       `json:"rating"`
	MessageID   int64     `json:"message_id"`
	SessionID   string    `json:"session_id"`
	CropID      string    `json:"crop_id"`
	TelegramID  int64     `json:"telegram_id"`
	Question    string    `json:"question"`
	Answer      string    `json:"answer"`
}

// FeedbackSummary — сводка 👍/👎.
type FeedbackSummary struct {
	Likes    int `json:"likes"`
	Dislikes int `json:"dislikes"`
}

// ListFeedbackReport возвращает оценки с парой вопрос/ответ (последний user перед assistant).
// ratingFilter: 0 — все, 1 или -1 — только этот рейтинг.
func (st *ChatStore) ListFeedbackReport(ctx context.Context, ratingFilter, limit int) ([]FeedbackReportItem, FeedbackSummary, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	summary, err := st.feedbackSummary(ctx)
	if err != nil {
		return nil, summary, err
	}
	ratingClause := ""
	args := []any{limit}
	if ratingFilter == 1 || ratingFilter == -1 {
		ratingClause = " AND mf.rating = $2 "
		args = []any{limit, ratingFilter}
	}
	query := `
		SELECT mf.created_at, mf.rating, mf.message_id, m_assist.session_id, cs.crop_id,
		       u.telegram_id, m_assist.content,
		       COALESCE((
		           SELECT m2.content
		           FROM messages m2
		           WHERE m2.session_id = m_assist.session_id
		             AND m2.role = 'user'
		             AND m2.id < m_assist.id
		           ORDER BY m2.id DESC
		           LIMIT 1
		       ), '') AS question
		FROM message_feedback mf
		JOIN messages m_assist ON m_assist.id = mf.message_id AND m_assist.role = 'assistant'
		JOIN chat_sessions cs ON cs.id = m_assist.session_id
		JOIN users u ON u.id = mf.user_id
		WHERE 1=1` + ratingClause + `
		ORDER BY mf.created_at DESC
		LIMIT $1`
	rows, err := st.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, summary, err
	}
	defer rows.Close()
	var items []FeedbackReportItem
	for rows.Next() {
		var it FeedbackReportItem
		if err := rows.Scan(
			&it.FeedbackAt, &it.Rating, &it.MessageID, &it.SessionID, &it.CropID,
			&it.TelegramID, &it.Answer, &it.Question,
		); err != nil {
			return nil, summary, err
		}
		items = append(items, it)
	}
	if items == nil {
		items = []FeedbackReportItem{}
	}
	return items, summary, rows.Err()
}

func (st *ChatStore) feedbackSummary(ctx context.Context) (FeedbackSummary, error) {
	var s FeedbackSummary
	err := st.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN rating = 1 THEN 1 ELSE 0 END), 0)::int,
			COALESCE(SUM(CASE WHEN rating = -1 THEN 1 ELSE 0 END), 0)::int
		FROM message_feedback`,
	).Scan(&s.Likes, &s.Dislikes)
	return s, err
}
