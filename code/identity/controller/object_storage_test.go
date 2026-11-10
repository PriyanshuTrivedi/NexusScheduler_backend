package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/store"
	chatbotpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
	chatbotmock "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/identity/client/chatbot"
	dbbytemock "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/identity/client/dbbyte"
	storemock "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/identity/store"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const (
	testUserID   = "11111111-1111-4111-8111-111111111111"
	otherUserID  = "22222222-2222-4222-8222-222222222222"
	testObjectID = "33333333-3333-4333-8333-333333333333"
	testKey      = "44444444-4444-4444-8444-444444444444"
)

func validProfilePictureRequest() entity.UploadFileRequest {
	return entity.UploadFileRequest{
		UserID: testUserID, FileName: "avatar.png", MIMEType: entity.MIMETypePNG,
		ObjectType: entity.ObjectTypeProfilePicture, Visibility: entity.ObjectVisibilityPublic,
		Content: []byte("image-content"),
	}
}

func validPDFRequest() entity.UploadFileRequest {
	return entity.UploadFileRequest{
		UserID: testUserID, FileName: "resume.pdf", MIMEType: entity.MIMETypePDF,
		ObjectType: entity.ObjectTypeResourceDocument, Visibility: entity.ObjectVisibilityPrivate,
		Content: []byte("%PDF-content"), IdempotencyKey: testKey,
	}
}

func TestUploadFile_ProfilePicture(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)

	db.EXPECT().Upload(gomock.Any(), gomock.Any(), entity.MIMETypePNG, []byte("image-content")).
		DoAndReturn(func(_ context.Context, name, _ string, content []byte) (entity.DBByteFile, error) {
			require.Contains(t, name, ".png")
			require.Equal(t, []byte("image-content"), content)
			return entity.DBByteFile{Success: true, Name: name, FileID: "dbbyte-1"}, nil
		})

	st.EXPECT().CreateObject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, object entity.ObjectMetadata) (entity.ObjectMetadata, error) {
			require.Equal(t, testUserID, object.UserID)
			require.Equal(t, entity.ObjectTypeProfilePicture, object.ObjectType)
			require.Equal(t, entity.ObjectVisibilityPublic, object.Visibility)
			require.Equal(t, entity.MIMETypePNG, object.MIMEType)
			require.Equal(t, int64(len("image-content")), object.SizeBytes)
			require.NotEmpty(t, object.ContentSHA256)
			object.ID = testObjectID
			return object, nil
		})

	c := NewObjectStorageController(st, db, chatbot)
	got, err := c.UploadFile(context.Background(), validProfilePictureRequest())

	require.NoError(t, err)
	require.Equal(t, testObjectID, got.Object.ID)
	require.False(t, got.Indexed)
	require.Nil(t, got.ChunksUpserted)
}

func TestUploadFile_ResourceDocumentIndexesAndMarksSynced(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	req := validPDFRequest()

	st.EXPECT().GetObjectByIdempotencyKey(gomock.Any(), testUserID, testKey).
		Return(entity.ObjectMetadata{}, store.ErrObjectNotFound)

	db.EXPECT().Upload(gomock.Any(), gomock.Any(), entity.MIMETypePDF, req.Content).
		DoAndReturn(func(_ context.Context, name, _ string, _ []byte) (entity.DBByteFile, error) {
			return entity.DBByteFile{Name: name, FileID: "dbbyte-pdf"}, nil
		})

	st.EXPECT().CreateObject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, object entity.ObjectMetadata) (entity.ObjectMetadata, error) {
			object.ID = testObjectID
			return object, nil
		})

	chatbot.EXPECT().IndexDocument(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request *chatbotpb.IndexDocumentRequest, _ ...grpc.CallOption) (*chatbotpb.IndexDocumentResponse, error) {
			require.Equal(t, testObjectID, request.ObjectId)
			require.Equal(t, testUserID, request.OwnerUserId)
			require.Equal(t, testObjectID, request.IdempotencyKey)
			require.Equal(t, req.Content, request.Content)
			return &chatbotpb.IndexDocumentResponse{Indexed: true, ChunksUpserted: 3}, nil
		})

	st.EXPECT().MarkObjectIndexed(gomock.Any(), testObjectID).Return(nil)

	c := NewObjectStorageController(st, db, chatbot)
	got, err := c.UploadFile(context.Background(), req)

	require.NoError(t, err)
	require.True(t, got.Indexed)
	require.NotNil(t, got.ChunksUpserted)
	require.Equal(t, int32(3), *got.ChunksUpserted)
	require.NotNil(t, got.Object.PineconeSyncedAt)
}

func TestUploadFile_InvalidRequestDoesNotCallDependencies(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := NewObjectStorageController(
		storemock.NewMockObjectStorageStore(ctrl),
		dbbytemock.NewMockClient(ctrl),
		chatbotmock.NewMockClient(ctrl),
	)

	req := validPDFRequest()
	req.Visibility = entity.ObjectVisibilityPublic

	_, err := c.UploadFile(context.Background(), req)
	require.ErrorIs(t, err, entity.ErrInvalidVisibility)
}

func TestUploadFile_IdempotencyConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)

	req := validPDFRequest()
	st.EXPECT().GetObjectByIdempotencyKey(gomock.Any(), testUserID, testKey).
		Return(entity.ObjectMetadata{ContentSHA256: "different-content"}, nil)

	c := NewObjectStorageController(st, db, chatbot)
	_, err := c.UploadFile(context.Background(), req)
	require.ErrorIs(t, err, entity.ErrIdempotencyConflict)
}

func TestUploadFile_ReturnsExistingIndexedObject(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)

	req := validPDFRequest()
	now := time.Now()
	object := entity.ObjectMetadata{
		ID: testObjectID, UserID: testUserID, ObjectType: entity.ObjectTypeResourceDocument,
		ContentSHA256: entity.ContentSHA256(req.Content), PineconeSyncedAt: &now,
	}
	st.EXPECT().GetObjectByIdempotencyKey(gomock.Any(), testUserID, testKey).Return(object, nil)

	c := NewObjectStorageController(st, db, chatbot)
	got, err := c.UploadFile(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, object, got.Object)
	require.True(t, got.Indexed)
}

func TestUploadFile_RetriesIndexingForExistingUnindexedObject(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)

	req := validPDFRequest()
	object := entity.ObjectMetadata{
		ID: testObjectID, UserID: testUserID, ObjectType: entity.ObjectTypeResourceDocument,
		ObjectKey: "stored.pdf", ContentSHA256: entity.ContentSHA256(req.Content),
	}
	st.EXPECT().GetObjectByIdempotencyKey(gomock.Any(), testUserID, testKey).Return(object, nil)
	db.EXPECT().Download(gomock.Any(), "stored.pdf").Return(req.Content, nil)
	chatbot.EXPECT().IndexDocument(gomock.Any(), gomock.Any()).
		Return(&chatbotpb.IndexDocumentResponse{Indexed: false}, nil)

	c := NewObjectStorageController(st, db, chatbot)
	got, err := c.UploadFile(context.Background(), req)
	require.NoError(t, err)
	require.False(t, got.Indexed)
}

func TestUploadFile_StoreErrorAfterDBByteUploadCleansUpFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	storeErr := errors.New("database unavailable")

	db.EXPECT().Upload(gomock.Any(), gomock.Any(), entity.MIMETypePNG, gomock.Any()).
		DoAndReturn(func(_ context.Context, name, _ string, _ []byte) (entity.DBByteFile, error) {
			return entity.DBByteFile{Name: name}, nil
		})
	st.EXPECT().CreateObject(gomock.Any(), gomock.Any()).Return(entity.ObjectMetadata{}, storeErr)
	db.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil)

	c := NewObjectStorageController(st, db, chatbot)
	_, err := c.UploadFile(context.Background(), validProfilePictureRequest())
	require.ErrorIs(t, err, storeErr)
}

func TestUploadFile_IdempotencyLookupError(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	dbErr := errors.New("database unavailable")

	req := validPDFRequest()
	st.EXPECT().GetObjectByIdempotencyKey(gomock.Any(), testUserID, testKey).
		Return(entity.ObjectMetadata{}, dbErr)

	c := NewObjectStorageController(st, db, chatbot)
	_, err := c.UploadFile(context.Background(), req)
	require.ErrorIs(t, err, dbErr)
}

func TestGetFile_PrivateOwnerCanDownload(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	object := entity.ObjectMetadata{
		ID: testObjectID, UserID: testUserID, ObjectKey: "avatar.png",
		Visibility: entity.ObjectVisibilityPrivate,
	}
	st.EXPECT().GetObject(gomock.Any(), testObjectID).Return(object, nil)
	db.EXPECT().Download(gomock.Any(), "avatar.png").Return([]byte("image"), nil)

	c := NewObjectStorageController(st, db, chatbot)
	got, err := c.GetFile(context.Background(), entity.GetFileRequest{CallerID: testUserID, ObjectID: testObjectID})
	require.NoError(t, err)
	require.Equal(t, []byte("image"), got.Content)
	require.Equal(t, object, got.Object)
}

func TestGetFile_PublicObjectCanBeDownloadedByAnotherUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	object := entity.ObjectMetadata{
		ID: testObjectID, UserID: testUserID, ObjectKey: "avatar.png",
		Visibility: entity.ObjectVisibilityPublic,
	}
	st.EXPECT().GetObject(gomock.Any(), testObjectID).Return(object, nil)
	db.EXPECT().Download(gomock.Any(), "avatar.png").Return([]byte("image"), nil)

	c := NewObjectStorageController(st, db, chatbot)
	got, err := c.GetFile(context.Background(), entity.GetFileRequest{CallerID: otherUserID, ObjectID: testObjectID})
	require.NoError(t, err)
	require.Equal(t, []byte("image"), got.Content)
}

func TestGetFile_PrivateObjectRejectsOtherUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	c := NewObjectStorageController(st, dbbytemock.NewMockClient(ctrl), chatbotmock.NewMockClient(ctrl))
	st.EXPECT().GetObject(gomock.Any(), testObjectID).
		Return(entity.ObjectMetadata{ID: testObjectID, UserID: testUserID, Visibility: entity.ObjectVisibilityPrivate}, nil)

	_, err := c.GetFile(context.Background(), entity.GetFileRequest{CallerID: otherUserID, ObjectID: testObjectID})
	require.ErrorIs(t, err, entity.ErrObjectForbidden)
}

func TestGetFile_InvalidIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := NewObjectStorageController(
		storemock.NewMockObjectStorageStore(ctrl),
		dbbytemock.NewMockClient(ctrl),
		chatbotmock.NewMockClient(ctrl),
	)
	_, err := c.GetFile(context.Background(), entity.GetFileRequest{CallerID: "bad", ObjectID: testObjectID})
	require.ErrorIs(t, err, entity.ErrInvalidObjectID)
}

func TestDeleteFile_ResourceDocumentDeletesIndexFileAndMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	object := entity.ObjectMetadata{
		ID: testObjectID, UserID: testUserID, ObjectType: entity.ObjectTypeResourceDocument,
		ObjectKey: "resume.pdf",
	}
	st.EXPECT().GetObject(gomock.Any(), testObjectID).Return(object, nil)
	chatbot.EXPECT().DeleteDocumentIndex(gomock.Any(), gomock.Any()).Return(&chatbotpb.DeleteDocumentIndexResponse{}, nil)
	db.EXPECT().Delete(gomock.Any(), "resume.pdf").Return(nil)
	st.EXPECT().DeleteObject(gomock.Any(), testObjectID).Return(nil)

	c := NewObjectStorageController(st, db, chatbot)
	got, err := c.DeleteFile(context.Background(), entity.DeleteFileRequest{CallerID: testUserID, ObjectID: testObjectID})
	require.NoError(t, err)
	require.True(t, got.Deleted)
}

func TestDeleteFile_ProfilePictureDoesNotDeleteChatbotIndex(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	st.EXPECT().GetObject(gomock.Any(), testObjectID).Return(entity.ObjectMetadata{
		ID: testObjectID, UserID: testUserID, ObjectType: entity.ObjectTypeProfilePicture, ObjectKey: "avatar.png",
	}, nil)
	db.EXPECT().Delete(gomock.Any(), "avatar.png").Return(nil)
	st.EXPECT().DeleteObject(gomock.Any(), testObjectID).Return(nil)

	c := NewObjectStorageController(st, db, chatbot)
	got, err := c.DeleteFile(context.Background(), entity.DeleteFileRequest{CallerID: testUserID, ObjectID: testObjectID})
	require.NoError(t, err)
	require.True(t, got.Deleted)
}

func TestDeleteFile_RejectsNonOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	c := NewObjectStorageController(st, dbbytemock.NewMockClient(ctrl), chatbotmock.NewMockClient(ctrl))
	st.EXPECT().GetObject(gomock.Any(), testObjectID).
		Return(entity.ObjectMetadata{ID: testObjectID, UserID: testUserID}, nil)

	_, err := c.DeleteFile(context.Background(), entity.DeleteFileRequest{CallerID: otherUserID, ObjectID: testObjectID})
	require.ErrorIs(t, err, entity.ErrObjectForbidden)
}

func TestDeleteFile_InvalidIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := NewObjectStorageController(
		storemock.NewMockObjectStorageStore(ctrl),
		dbbytemock.NewMockClient(ctrl),
		chatbotmock.NewMockClient(ctrl),
	)
	_, err := c.DeleteFile(context.Background(), entity.DeleteFileRequest{CallerID: testUserID, ObjectID: "bad"})
	require.ErrorIs(t, err, entity.ErrInvalidObjectID)
}

func TestDeleteFile_DBByteFailureKeepsMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	dbErr := errors.New("dbbyte unavailable")
	st.EXPECT().GetObject(gomock.Any(), testObjectID).Return(entity.ObjectMetadata{
		ID: testObjectID, UserID: testUserID, ObjectType: entity.ObjectTypeProfilePicture, ObjectKey: "avatar.png",
	}, nil)
	db.EXPECT().Delete(gomock.Any(), "avatar.png").Return(dbErr)

	c := NewObjectStorageController(st, db, chatbot)
	_, err := c.DeleteFile(context.Background(), entity.DeleteFileRequest{CallerID: testUserID, ObjectID: testObjectID})
	require.ErrorIs(t, err, dbErr)
}

func TestUploadFile_DBByteUploadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	uploadErr := errors.New("dbbyte upload failed")
	db.EXPECT().Upload(gomock.Any(), gomock.Any(), entity.MIMETypePNG, gomock.Any()).
		Return(entity.DBByteFile{}, uploadErr)

	c := NewObjectStorageController(st, db, chatbot)
	_, err := c.UploadFile(context.Background(), validProfilePictureRequest())
	require.ErrorIs(t, err, uploadErr)
}

func TestUploadFile_ChatbotIndexError(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	req := validPDFRequest()

	st.EXPECT().GetObjectByIdempotencyKey(gomock.Any(), testUserID, testKey).
		Return(entity.ObjectMetadata{}, store.ErrObjectNotFound)
	db.EXPECT().Upload(gomock.Any(), gomock.Any(), entity.MIMETypePDF, req.Content).
		DoAndReturn(func(_ context.Context, name, _ string, _ []byte) (entity.DBByteFile, error) {
			return entity.DBByteFile{Name: name}, nil
		})
	st.EXPECT().CreateObject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, object entity.ObjectMetadata) (entity.ObjectMetadata, error) {
			object.ID = testObjectID
			return object, nil
		})
	indexErr := errors.New("chatbot unavailable")
	chatbot.EXPECT().IndexDocument(gomock.Any(), gomock.Any()).
		Return(nil, indexErr)

	c := NewObjectStorageController(st, db, chatbot)
	_, err := c.UploadFile(context.Background(), req)
	require.ErrorIs(t, err, indexErr)
}

func TestUploadFile_MarkIndexedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	req := validPDFRequest()

	st.EXPECT().GetObjectByIdempotencyKey(gomock.Any(), testUserID, testKey).
		Return(entity.ObjectMetadata{}, store.ErrObjectNotFound)
	db.EXPECT().Upload(gomock.Any(), gomock.Any(), entity.MIMETypePDF, req.Content).
		DoAndReturn(func(_ context.Context, name, _ string, _ []byte) (entity.DBByteFile, error) {
			return entity.DBByteFile{Name: name}, nil
		})
	st.EXPECT().CreateObject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, object entity.ObjectMetadata) (entity.ObjectMetadata, error) {
			object.ID = testObjectID
			return object, nil
		})
	chatbot.EXPECT().IndexDocument(gomock.Any(), gomock.Any()).
		Return(&chatbotpb.IndexDocumentResponse{Indexed: true, ChunksUpserted: 2}, nil)
	markErr := errors.New("metadata update failed")
	st.EXPECT().MarkObjectIndexed(gomock.Any(), testObjectID).Return(markErr)

	c := NewObjectStorageController(st, db, chatbot)
	_, err := c.UploadFile(context.Background(), req)
	require.ErrorIs(t, err, markErr)
}

func TestUploadFile_ExistingObjectDownloadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	st := storemock.NewMockObjectStorageStore(ctrl)
	db := dbbytemock.NewMockClient(ctrl)
	chatbot := chatbotmock.NewMockClient(ctrl)
	req := validPDFRequest()
	object := entity.ObjectMetadata{
		ID: testObjectID, UserID: testUserID, ObjectType: entity.ObjectTypeResourceDocument,
		ObjectKey: "stored.pdf", ContentSHA256: entity.ContentSHA256(req.Content),
	}
	st.EXPECT().GetObjectByIdempotencyKey(gomock.Any(), testUserID, testKey).Return(object, nil)
	downloadErr := errors.New("dbbyte download failed")
	db.EXPECT().Download(gomock.Any(), "stored.pdf").Return(nil, downloadErr)

	c := NewObjectStorageController(st, db, chatbot)
	_, err := c.UploadFile(context.Background(), req)
	require.ErrorIs(t, err, downloadErr)
}

func TestGetFile_StoreAndDBByteErrors(t *testing.T) {
	t.Run("store error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		st := storemock.NewMockObjectStorageStore(ctrl)
		stErr := errors.New("database unavailable")
		st.EXPECT().GetObject(gomock.Any(), testObjectID).Return(entity.ObjectMetadata{}, stErr)

		c := NewObjectStorageController(st, dbbytemock.NewMockClient(ctrl), chatbotmock.NewMockClient(ctrl))
		_, err := c.GetFile(context.Background(), entity.GetFileRequest{CallerID: testUserID, ObjectID: testObjectID})
		require.ErrorIs(t, err, stErr)
	})

	t.Run("dbbyte error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		st := storemock.NewMockObjectStorageStore(ctrl)
		db := dbbytemock.NewMockClient(ctrl)
		st.EXPECT().GetObject(gomock.Any(), testObjectID).Return(entity.ObjectMetadata{
			ID: testObjectID, UserID: testUserID, ObjectKey: "avatar.png", Visibility: entity.ObjectVisibilityPrivate,
		}, nil)
		dbErr := errors.New("dbbyte unavailable")
		db.EXPECT().Download(gomock.Any(), "avatar.png").Return(nil, dbErr)

		c := NewObjectStorageController(st, db, chatbotmock.NewMockClient(ctrl))
		_, err := c.GetFile(context.Background(), entity.GetFileRequest{CallerID: testUserID, ObjectID: testObjectID})
		require.ErrorIs(t, err, dbErr)
	})
}

func TestDeleteFile_ChatbotAndMetadataErrors(t *testing.T) {
	t.Run("chatbot error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		st := storemock.NewMockObjectStorageStore(ctrl)
		chatbot := chatbotmock.NewMockClient(ctrl)
		st.EXPECT().GetObject(gomock.Any(), testObjectID).Return(entity.ObjectMetadata{
			ID: testObjectID, UserID: testUserID, ObjectType: entity.ObjectTypeResourceDocument,
			ObjectKey: "resume.pdf",
		}, nil)
		deleteErr := errors.New("chatbot unavailable")
		chatbot.EXPECT().DeleteDocumentIndex(gomock.Any(), gomock.Any()).Return(nil, deleteErr)

		c := NewObjectStorageController(st, dbbytemock.NewMockClient(ctrl), chatbot)
		_, err := c.DeleteFile(context.Background(), entity.DeleteFileRequest{CallerID: testUserID, ObjectID: testObjectID})
		require.ErrorIs(t, err, deleteErr)
	})

	t.Run("metadata error after object deletion", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		st := storemock.NewMockObjectStorageStore(ctrl)
		db := dbbytemock.NewMockClient(ctrl)
		st.EXPECT().GetObject(gomock.Any(), testObjectID).Return(entity.ObjectMetadata{
			ID: testObjectID, UserID: testUserID, ObjectType: entity.ObjectTypeProfilePicture, ObjectKey: "avatar.png",
		}, nil)
		db.EXPECT().Delete(gomock.Any(), "avatar.png").Return(nil)
		deleteErr := errors.New("metadata delete failed")
		st.EXPECT().DeleteObject(gomock.Any(), testObjectID).Return(deleteErr)

		c := NewObjectStorageController(st, db, chatbotmock.NewMockClient(ctrl))
		_, err := c.DeleteFile(context.Background(), entity.DeleteFileRequest{CallerID: testUserID, ObjectID: testObjectID})
		require.ErrorIs(t, err, deleteErr)
	})
}
