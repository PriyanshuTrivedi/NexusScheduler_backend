package store

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type scanRow struct {
	values []any
	err    error
}

func (r scanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("scan destination count mismatch")
	}
	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return errors.New("scan destination must be a non-nil pointer")
		}
		value := reflect.ValueOf(r.values[i])
		if !value.IsValid() {
			target.Elem().Set(reflect.Zero(target.Elem().Type()))
			continue
		}
		if value.Type().AssignableTo(target.Elem().Type()) {
			target.Elem().Set(value)
			continue
		}
		if value.Type().ConvertibleTo(target.Elem().Type()) {
			target.Elem().Set(value.Convert(target.Elem().Type()))
			continue
		}
		return errors.New("scan value type mismatch")
	}
	return nil
}

func TestScanObject_Success(t *testing.T) {
	createdAt := time.Unix(100, 0)
	updatedAt := time.Unix(200, 0)
	syncedAt := time.Unix(150, 0)

	got, err := scanObject(scanRow{values: []any{
		"object-1", "user-1", entity.ObjectTypeResourceDocument, entity.ObjectVisibilityPrivate,
		"stored.pdf", "dbbyte-1", "resume.pdf", entity.MIMETypePDF, int64(123),
		"44444444-4444-4444-8444-444444444444", "0123456789abcdef", &syncedAt,
		createdAt, updatedAt,
	}})
	require.NoError(t, err)
	require.Equal(t, "object-1", got.ID)
	require.Equal(t, "user-1", got.UserID)
	require.Equal(t, entity.ObjectTypeResourceDocument, got.ObjectType)
	require.Equal(t, entity.ObjectVisibilityPrivate, got.Visibility)
	require.Equal(t, "stored.pdf", got.ObjectKey)
	require.Equal(t, "dbbyte-1", got.DBByteFileID)
	require.Equal(t, "resume.pdf", got.OriginalFileName)
	require.Equal(t, entity.MIMETypePDF, got.MIMEType)
	require.Equal(t, int64(123), got.SizeBytes)
	require.Equal(t, "44444444-4444-4444-8444-444444444444", got.IdempotencyKey)
	require.Equal(t, &syncedAt, got.PineconeSyncedAt)
	require.Equal(t, createdAt, got.CreatedAt)
	require.Equal(t, updatedAt, got.UpdatedAt)
}

func TestScanObject_NullableFields(t *testing.T) {
	createdAt := time.Unix(100, 0)
	updatedAt := time.Unix(200, 0)

	got, err := scanObject(scanRow{values: []any{
		"object-1", "user-1", entity.ObjectTypeProfilePicture, entity.ObjectVisibilityPublic,
		"avatar.png", "", "avatar.png", entity.MIMETypePNG, int64(10),
		"", "0123456789abcdef", (*time.Time)(nil), createdAt, updatedAt,
	}})
	require.NoError(t, err)
	require.Empty(t, got.DBByteFileID)
	require.Empty(t, got.IdempotencyKey)
	require.Nil(t, got.PineconeSyncedAt)
}

func TestScanObject_NotFound(t *testing.T) {
	got, err := scanObject(scanRow{err: pgx.ErrNoRows})
	require.ErrorIs(t, err, ErrObjectNotFound)
	require.Equal(t, entity.ObjectMetadata{}, got)
}

func TestScanObject_ScanError(t *testing.T) {
	scanErr := errors.New("scan failed")
	got, err := scanObject(scanRow{err: scanErr})
	require.ErrorIs(t, err, scanErr)
	require.Contains(t, err.Error(), "store: scan object")
	require.Equal(t, entity.ObjectMetadata{}, got)
}
