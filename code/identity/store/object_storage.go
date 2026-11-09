package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:generate mockgen -source=object_storage.go -destination=../../../gen/mocks/identity/store/object_storage_mock.go -package=mocks

var (
	ErrObjectAlreadyExists = errors.New("store: object already exists")
	ErrObjectNotFound      = errors.New("store: object not found")
)

type ObjectStorageStore interface {
	CreateObject(ctx context.Context, object entity.ObjectMetadata) (entity.ObjectMetadata, error)
	GetObject(ctx context.Context, id string) (entity.ObjectMetadata, error)
	GetObjectByIdempotencyKey(ctx context.Context, userID, key string) (entity.ObjectMetadata, error)
	MarkObjectIndexed(ctx context.Context, id string) error
	DeleteObject(ctx context.Context, id string) error
}

type pgObjectStorageStore struct {
	pool *pgxpool.Pool
}

func NewObjectStorageStore(pool *pgxpool.Pool) ObjectStorageStore {
	return &pgObjectStorageStore{
		pool: pool,
	}
}

func (s *pgObjectStorageStore) CreateObject(ctx context.Context, object entity.ObjectMetadata) (entity.ObjectMetadata, error) {
	err := s.pool.QueryRow(
		ctx,
		`INSERT INTO object_storage (
			user_id,
			object_type,
			visibility,
			object_key,
			dbbyte_file_id,
			original_file_name,
			mime_type,
			size_bytes,
			idempotency_key,
			content_sha256
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id::text, created_at, updated_at`,
		object.UserID,
		object.ObjectType,
		object.Visibility,
		object.ObjectKey,
		object.DBByteFileID,
		object.OriginalFileName,
		object.MIMEType,
		object.SizeBytes,
		object.IdempotencyKey,
		object.ContentSHA256,
	).Scan(&object.ID, &object.CreatedAt, &object.UpdatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			return entity.ObjectMetadata{}, ErrObjectAlreadyExists
		}

		return entity.ObjectMetadata{}, fmt.Errorf("store: create object: %w", err)
	}

	return object, nil
}

func (s *pgObjectStorageStore) GetObject(ctx context.Context, id string) (entity.ObjectMetadata, error) {
	return scanObject(
		s.pool.QueryRow(
			ctx,
			`SELECT `+entity.ObjectStorageColumns+` FROM object_storage WHERE id = $1`,
			id,
		),
	)
}

func (s *pgObjectStorageStore) GetObjectByIdempotencyKey(ctx context.Context, userID, key string) (entity.ObjectMetadata, error) {
	return scanObject(
		s.pool.QueryRow(
			ctx,
			`SELECT `+entity.ObjectStorageColumns+` FROM object_storage WHERE user_id = $1 AND idempotency_key = $2`,
			userID,
			key,
		),
	)
}

func (s *pgObjectStorageStore) MarkObjectIndexed(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(
		ctx,
		`UPDATE object_storage
		SET pinecone_synced_at = now(), updated_at = now()
		WHERE id = $1`,
		id,
	)

	if err != nil {
		return fmt.Errorf("store: mark indexed: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrObjectNotFound
	}

	return nil
}

func (s *pgObjectStorageStore) DeleteObject(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(
		ctx,
		`DELETE FROM object_storage WHERE id = $1`,
		id,
	)

	if err != nil {
		return fmt.Errorf("store: delete object: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrObjectNotFound
	}

	return nil
}

func scanObject(row pgx.Row) (entity.ObjectMetadata, error) {
	var object entity.ObjectMetadata

	err := row.Scan(
		&object.ID,
		&object.UserID,
		&object.ObjectType,
		&object.Visibility,
		&object.ObjectKey,
		&object.DBByteFileID,
		&object.OriginalFileName,
		&object.MIMEType,
		&object.SizeBytes,
		&object.IdempotencyKey,
		&object.ContentSHA256,
		&object.PineconeSyncedAt,
		&object.CreatedAt,
		&object.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ObjectMetadata{}, ErrObjectNotFound
	}

	if err != nil {
		return entity.ObjectMetadata{}, fmt.Errorf("store: scan object: %w", err)
	}

	return object, nil
}
