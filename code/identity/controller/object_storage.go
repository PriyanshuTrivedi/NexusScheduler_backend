package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	chatbotclient "github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/client/chatbot"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/client/dbbyte"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/store"
	chatbotpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
)

//go:generate mockgen -source=object_storage.go -destination=../../../gen/mocks/identity/controller/object_storage_mock.go -package=mocks

type ObjectStorageController interface {
	UploadFile(context.Context, entity.UploadFileRequest) (entity.UploadFileResponse, error)
	GetFile(context.Context, entity.GetFileRequest) (entity.GetFileResponse, error)
	DeleteFile(context.Context, entity.DeleteFileRequest) (entity.DeleteFileResponse, error)
}

type objectStorageController struct {
	store   store.ObjectStorageStore
	dbbyte  dbbyte.Client
	chatbot chatbotclient.Client
}

func NewObjectStorageController(s store.ObjectStorageStore, d dbbyte.Client, c chatbotclient.Client) ObjectStorageController {
	return &objectStorageController{
		store:   s,
		dbbyte:  d,
		chatbot: c,
	}
}

func (c *objectStorageController) UploadFile(ctx context.Context, req entity.UploadFileRequest) (entity.UploadFileResponse, error) {
	if err := req.NormalizeAndValidate(); err != nil {
		return entity.UploadFileResponse{}, err
	}

	contentSHA256 := entity.ContentSHA256(req.Content)

	if req.IdempotencyKey != "" {
		existing, err := c.store.GetObjectByIdempotencyKey(ctx, req.UserID, req.IdempotencyKey)
		if err != nil && !errors.Is(err, store.ErrObjectNotFound) {
			return entity.UploadFileResponse{}, fmt.Errorf("get object by idempotency key: %w", err)
		}

		if err == nil {
			return c.handleExistingUpload(ctx, existing, contentSHA256)
		}
	}

	objectKey, err := generateObjectKey(req.FileName)
	if err != nil {
		return entity.UploadFileResponse{}, err
	}

	file, err := c.dbbyte.Upload(ctx, objectKey, req.MIMEType, req.Content)
	if err != nil {
		return entity.UploadFileResponse{}, fmt.Errorf("identity: upload file to dbbyte: %w", err)
	}

	if file.Name != "" {
		objectKey = file.Name
	}

	object := entity.ObjectMetadata{
		UserID:           req.UserID,
		ObjectType:       req.ObjectType,
		Visibility:       req.Visibility,
		ObjectKey:        objectKey,
		DBByteFileID:     file.FileID,
		OriginalFileName: req.FileName,
		MIMEType:         req.MIMEType,
		SizeBytes:        int64(len(req.Content)),
		IdempotencyKey:   req.IdempotencyKey,
		ContentSHA256:    contentSHA256,
	}

	object, err = c.store.CreateObject(ctx, object)
	if err != nil {
		// Avoid leaving an uploaded file without corresponding metadata.
		_ = c.dbbyte.Delete(ctx, objectKey)
		return entity.UploadFileResponse{}, fmt.Errorf("identity: create object metadata: %w", err)
	}

	return c.indexDocument(ctx, object, req.Content)
}

func (c *objectStorageController) handleExistingUpload(ctx context.Context, object entity.ObjectMetadata, contentSHA256 string) (entity.UploadFileResponse, error) {
	if object.ContentSHA256 != contentSHA256 {
		return entity.UploadFileResponse{}, entity.ErrIdempotencyConflict
	}

	if object.ObjectType != entity.ObjectTypeResourceDocument || object.PineconeSyncedAt != nil {
		return entity.UploadFileResponse{
			Object:  object,
			Indexed: object.PineconeSyncedAt != nil,
		}, nil
	}

	content, err := c.dbbyte.Download(ctx, object.ObjectKey)
	if err != nil {
		return entity.UploadFileResponse{}, fmt.Errorf("identity: download existing file for indexing: %w", err)
	}

	return c.indexDocument(ctx, object, content)
}

func (c *objectStorageController) indexDocument(ctx context.Context, object entity.ObjectMetadata, content []byte) (entity.UploadFileResponse, error) {
	if object.ObjectType != entity.ObjectTypeResourceDocument {
		return entity.UploadFileResponse{
			Object:  object,
			Indexed: false,
		}, nil
	}

	response, err := c.chatbot.IndexDocument(ctx, &chatbotpb.IndexDocumentRequest{
		ObjectId:       object.ID,
		OwnerUserId:    object.UserID,
		IdempotencyKey: object.ID,
		DocumentType:   chatbotpb.DocumentType_DOCUMENT_TYPE_PDF,
		Content:        content,
	})
	if err != nil {
		return entity.UploadFileResponse{}, fmt.Errorf("identity: index document in chatbot: %w", err)
	}

	if !response.GetIndexed() {
		return entity.UploadFileResponse{
			Object:  object,
			Indexed: false,
		}, nil
	}

	if err := c.store.MarkObjectIndexed(ctx, object.ID); err != nil {
		return entity.UploadFileResponse{}, fmt.Errorf("identity: mark object indexed: %w", err)
	}

	now := time.Now()
	object.PineconeSyncedAt = &now

	chunksUpserted := response.GetChunksUpserted()

	return entity.UploadFileResponse{
		Object:         object,
		Indexed:        true,
		ChunksUpserted: &chunksUpserted,
	}, nil
}

func (c *objectStorageController) GetFile(ctx context.Context, req entity.GetFileRequest) (entity.GetFileResponse, error) {
	if !entity.IsUUID(req.CallerID) || !entity.IsUUID(req.ObjectID) {
		return entity.GetFileResponse{}, entity.ErrInvalidObjectID
	}

	object, err := c.store.GetObject(ctx, req.ObjectID)
	if err != nil {
		return entity.GetFileResponse{}, fmt.Errorf("identity: get object: %w", err)
	}

	if object.Visibility == entity.ObjectVisibilityPrivate && object.UserID != req.CallerID {
		return entity.GetFileResponse{}, entity.ErrObjectForbidden
	}

	content, err := c.dbbyte.Download(ctx, object.ObjectKey)
	if err != nil {
		return entity.GetFileResponse{}, fmt.Errorf("identity: download file from dbbyte: %w", err)
	}

	return entity.GetFileResponse{
		Object:  object,
		Content: content,
	}, nil
}

func (c *objectStorageController) DeleteFile(ctx context.Context, req entity.DeleteFileRequest) (entity.DeleteFileResponse, error) {
	if !entity.IsUUID(req.CallerID) || !entity.IsUUID(req.ObjectID) {
		return entity.DeleteFileResponse{}, entity.ErrInvalidObjectID
	}

	object, err := c.store.GetObject(ctx, req.ObjectID)
	if err != nil {
		return entity.DeleteFileResponse{}, fmt.Errorf("identity: get object for deletion: %w", err)
	}

	// Only the owner can delete an object, regardless of its visibility.
	if object.UserID != req.CallerID {
		return entity.DeleteFileResponse{}, entity.ErrObjectForbidden
	}

	if object.ObjectType == entity.ObjectTypeResourceDocument {
		_, err := c.chatbot.DeleteDocumentIndex(ctx, &chatbotpb.DeleteDocumentIndexRequest{
			ObjectId:    object.ID,
			OwnerUserId: object.UserID,
		})
		if err != nil {
			return entity.DeleteFileResponse{}, fmt.Errorf("identity: delete document index: %w", err)
		}
	}

	if err := c.dbbyte.Delete(ctx, object.ObjectKey); err != nil {
		return entity.DeleteFileResponse{}, fmt.Errorf("identity: delete file from dbbyte: %w", err)
	}

	if err := c.store.DeleteObject(ctx, object.ID); err != nil {
		return entity.DeleteFileResponse{}, fmt.Errorf("identity: delete object metadata: %w", err)
	}

	return entity.DeleteFileResponse{Deleted: true}, nil
}

func generateObjectKey(fileName string) (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("identity: generate object key: %w", err)
	}

	return hex.EncodeToString(randomBytes) + filepath.Ext(fileName), nil
}
