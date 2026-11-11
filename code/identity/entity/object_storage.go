package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	ObjectVisibilityPublic  = "PUBLIC"
	ObjectVisibilityPrivate = "PRIVATE"

	ObjectTypeProfilePicture   = "PROFILE_PICTURE"
	ObjectTypeResourceDocument = "RESOURCE_DOCUMENT"

	MIMETypePDF  = "application/pdf"
	MIMETypeJPEG = "image/jpeg"
	MIMETypePNG  = "image/png"
	MIMETypeWEBP = "image/webp"

	MaxPDFSizeBytes = 1 << 20
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsUUID(value string) bool { return uuidPattern.MatchString(value) }

var (
	ErrInvalidObjectID     = errors.New("identity: invalid object_id")
	ErrInvalidObjectType   = errors.New("identity: unsupported object_type")
	ErrInvalidVisibility   = errors.New("identity: invalid visibility")
	ErrInvalidFile         = errors.New("identity: invalid file")
	ErrObjectForbidden     = errors.New("identity: object access forbidden")
	ErrIdempotencyConflict = errors.New("identity: idempotency key reused with different content")
)

type ObjectMetadata struct {
	ID               string
	UserID           string
	ObjectType       string
	Visibility       string
	ObjectKey        string
	DBByteFileID     string
	OriginalFileName string
	MIMEType         string
	SizeBytes        int64
	IdempotencyKey   string
	ContentSHA256    string
	PineconeSyncedAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type UploadFileRequest struct {
	UserID         string
	FileName       string
	MIMEType       string
	ObjectType     string
	Visibility     string
	Content        []byte
	IdempotencyKey string
}

type UploadFileResponse struct {
	Object         ObjectMetadata
	Indexed        bool
	ChunksUpserted *int32
}

type GetFileRequest struct {
	CallerID string
	ObjectID string
}

type GetFileResponse struct {
	Object  ObjectMetadata
	Content []byte
}

type DeleteFileRequest struct {
	CallerID string
	ObjectID string
}

type DeleteFileResponse struct {
	Deleted bool
}

type ListFilesRequest struct {
	CallerID   string
	ObjectType string
}

type ListFilesResponse struct {
	Objects []ObjectMetadata
}

func (req *UploadFileRequest) NormalizeAndValidate() error {
	req.UserID = strings.TrimSpace(req.UserID)
	req.FileName = strings.TrimSpace(req.FileName)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	req.ObjectType = strings.ToUpper(strings.TrimSpace(req.ObjectType))
	req.Visibility = strings.ToUpper(strings.TrimSpace(req.Visibility))
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)

	if !IsUUID(req.UserID) ||
		req.FileName == "" ||
		req.MIMEType == "" ||
		len(req.Content) == 0 ||
		(req.IdempotencyKey != "" && !IsUUID(req.IdempotencyKey)) {
		return ErrInvalidFile
	}

	if req.ObjectType != ObjectTypeProfilePicture && req.ObjectType != ObjectTypeResourceDocument {
		return ErrInvalidObjectType
	}

	if req.Visibility != ObjectVisibilityPublic && req.Visibility != ObjectVisibilityPrivate {
		return ErrInvalidVisibility
	}

	if req.ObjectType == ObjectTypeProfilePicture {
		if req.Visibility != ObjectVisibilityPublic {
			return ErrInvalidVisibility
		}
		if !isImageMIMEType(req.MIMEType) {
			return ErrInvalidFile
		}
	}

	if req.ObjectType == ObjectTypeResourceDocument {
		if req.Visibility != ObjectVisibilityPrivate {
			return ErrInvalidVisibility
		}
		if req.MIMEType != MIMETypePDF || int64(len(req.Content)) > MaxPDFSizeBytes {
			return ErrInvalidFile
		}
	}

	return nil
}

func isImageMIMEType(mimeType string) bool {
	switch mimeType {
	case MIMETypeJPEG, MIMETypePNG, MIMETypeWEBP:
		return true
	default:
		return false
	}
}

func ContentSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

const ObjectStorageColumns = `
	id::text,
	user_id::text,
	object_type,
	visibility,
	object_key,
	COALESCE(dbbyte_file_id, ''),
	original_file_name,
	mime_type,
	size_bytes,
	COALESCE(idempotency_key::text, ''),
	content_sha256,
	pinecone_synced_at,
	created_at,
	updated_at
`
