package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
)

const (
	tableConversation = "conversation"
	tableMessage      = "message"
	previewMaxLength  = 200
)

var (
	ErrConversationNotFound = errors.New("store: conversation not found")
	ErrConversationClosed   = errors.New("store: conversation is closed")
)

//go:generate mockgen -source=chat.go -destination=../../../gen/mocks/chatbot/store/chat_mock.go -package=mocks

type Store interface {
	GetActiveConversation(ctx context.Context, userID string) (*entity.Conversation, error)
	CreateConversation(ctx context.Context, userID string) (*entity.Conversation, error)
	CloseActiveConversation(ctx context.Context, userID string, reason entity.CloseReason) (*entity.Conversation, error)
	CreateMessage(ctx context.Context, conversationID string, msg entity.Message, idempotencyKey string) (*entity.Message, error)
	GetMessages(ctx context.Context, userID, conversationID string) ([]*entity.Message, error)
	ListConversations(ctx context.Context, userID string, limit, offset int32) ([]*entity.Conversation, error)
}

type pgStore struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) Store {
	return &pgStore{pool: pool}
}

func (s *pgStore) GetActiveConversation(ctx context.Context, userID string) (*entity.Conversation, error) {
	var c entity.Conversation
	var status, closeReason string

	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, user_id, title, last_message_preview, status,
		       COALESCE(close_reason, ''), created_at, updated_at
		FROM %s
		WHERE user_id = $1 AND status = $2
	`, tableConversation), userID, entity.StatusActive).Scan(
		&c.ID, &c.UserID, &c.Title, &c.LastMessagePreview,
		&status, &closeReason, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get active conversation: %w", err)
	}

	c.Status = entity.ConversationStatus(status)
	if closeReason != "" {
		reason := entity.CloseReason(closeReason)
		c.CloseReason = &reason
	}

	return &c, nil
}

func (s *pgStore) CreateConversation(ctx context.Context, userID string) (*entity.Conversation, error) {
	var c entity.Conversation

	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s (user_id)
		VALUES ($1)
		RETURNING id, user_id, title, last_message_preview, status,
		          created_at, updated_at
	`, tableConversation), userID).Scan(
		&c.ID, &c.UserID, &c.Title, &c.LastMessagePreview,
		&c.Status, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("store: create conversation: %w", err)
	}

	return &c, nil
}

func (s *pgStore) CloseActiveConversation(ctx context.Context, userID string, reason entity.CloseReason) (*entity.Conversation, error) {
	var c entity.Conversation

	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s
		SET status = $1, close_reason = $2, updated_at = now()
		WHERE user_id = $3 AND status = $4
		RETURNING id, user_id, title, last_message_preview, status,
		          close_reason, created_at, updated_at
	`, tableConversation), entity.StatusClosed, reason, userID, entity.StatusActive).Scan(
		&c.ID, &c.UserID, &c.Title, &c.LastMessagePreview,
		&c.Status, &c.CloseReason, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: close active conversation: %w", err)
	}

	return &c, nil
}

func (s *pgStore) CreateMessage(ctx context.Context, conversationID string, msg entity.Message, idempotencyKey string) (*entity.Message, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: begin create message transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var created entity.Message

	var key any
	if idempotencyKey != "" {
		key = idempotencyKey
	}

	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s (conversation_id, role, content, client_idempotency_key)
		SELECT $1, $2, $3, $4
		WHERE EXISTS (
			SELECT 1 FROM %s
			WHERE id = $1 AND status = $5
		)
		RETURNING id, role, content, created_at
	`, tableMessage, tableConversation),
		conversationID, string(msg.Role), msg.Content, key, entity.StatusActive,
	).Scan(&created.ID, &created.Role, &created.Content, &created.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		var status string
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT status FROM %s WHERE id = $1
		`, tableConversation), conversationID).Scan(&status)

		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConversationNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("store: check conversation status: %w", err)
		}
		return nil, ErrConversationClosed
	}
	if err != nil {
		return nil, fmt.Errorf("store: create message: %w", err)
	}

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET last_message_preview = LEFT($1, $2), updated_at = now()
		WHERE id = $3
	`, tableConversation), msg.Content, previewMaxLength, conversationID)
	if err != nil {
		return nil, fmt.Errorf("store: update conversation activity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("store: commit create message transaction: %w", err)
	}

	created.Role = msg.Role
	return &created, nil
}

func (s *pgStore) GetMessages(ctx context.Context, userID, conversationID string) ([]*entity.Message, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT m.id, m.role, m.content, m.created_at
		FROM %s m
		INNER JOIN %s c ON c.id = m.conversation_id
		WHERE c.id = $1 AND c.user_id = $2
		ORDER BY m.created_at ASC, m.id ASC
	`, tableMessage, tableConversation), conversationID, userID)
	if err != nil {
		return nil, fmt.Errorf("store: get conversation messages: %w", err)
	}
	defer rows.Close()

	messages := make([]*entity.Message, 0)
	for rows.Next() {
		var msg entity.Message
		var role string

		if err := rows.Scan(&msg.ID, &role, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scan conversation message: %w", err)
		}

		msg.Role = entity.Role(role)
		messages = append(messages, &msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: get conversation messages rows: %w", err)
	}

	return messages, nil
}

func (s *pgStore) ListConversations(ctx context.Context, userID string, limit, offset int32) ([]*entity.Conversation, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, user_id, title, last_message_preview, status,
		       COALESCE(close_reason, ''), created_at, updated_at
		FROM %s
		WHERE user_id = $1
		ORDER BY updated_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, tableConversation), userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("store: list conversations: %w", err)
	}
	defer rows.Close()

	conversations := make([]*entity.Conversation, 0)
	for rows.Next() {
		var c entity.Conversation
		var status, closeReason string

		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Title, &c.LastMessagePreview,
			&status, &closeReason, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("store: scan conversation: %w", err)
		}

		c.Status = entity.ConversationStatus(status)
		if closeReason != "" {
			reason := entity.CloseReason(closeReason)
			c.CloseReason = &reason
		}

		conversations = append(conversations, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list conversations rows: %w", err)
	}

	return conversations, nil
}
