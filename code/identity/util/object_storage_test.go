package util

import (
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	pb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	"github.com/stretchr/testify/require"
)

const (
	testUserIDForUtil         = "11111111-1111-4111-8111-111111111111"
	testIdempotencyKeyForUtil = "44444444-4444-4444-8444-444444444444"
)

func TestUploadFileRequestToEntity(t *testing.T) {
	idempotencyKey := testIdempotencyKeyForUtil

	req := &pb.UploadFileRequest{
		UserId:         testUserIDForUtil,
		FileName:       "resume.pdf",
		MimeType:       pb.MimeType_MIME_TYPE_PDF,
		ObjectType:     pb.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT,
		Visibility:     pb.ObjectVisibility_OBJECT_VISIBILITY_PRIVATE,
		Content:        []byte("pdf"),
		IdempotencyKey: &idempotencyKey,
	}

	got, err := UploadFileRequestToEntity(req)
	require.NoError(t, err)
	require.Equal(t, entity.UploadFileRequest{
		UserID: testUserIDForUtil, FileName: "resume.pdf", MIMEType: entity.MIMETypePDF,
		ObjectType: entity.ObjectTypeResourceDocument, Visibility: entity.ObjectVisibilityPrivate,
		Content: []byte("pdf"), IdempotencyKey: testIdempotencyKeyForUtil,
	}, got)
}

func TestUploadFileResponseToProto(t *testing.T) {
	now := time.Unix(100, 0)
	chunks := int32(4)
	got := UploadFileResponseToProto(entity.UploadFileResponse{
		Object: entity.ObjectMetadata{
			ID: "object-1", UserID: testUserIDForUtil, ObjectType: entity.ObjectTypeResourceDocument,
			Visibility: entity.ObjectVisibilityPrivate, OriginalFileName: "resume.pdf",
			MIMEType: entity.MIMETypePDF, SizeBytes: 123, CreatedAt: now, UpdatedAt: now,
		},
		Indexed: true, ChunksUpserted: &chunks,
	})

	require.Equal(t, "object-1", got.Object.ObjectId)
	require.Equal(t, pb.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT, got.Object.ObjectType)
	require.Equal(t, pb.ObjectVisibility_OBJECT_VISIBILITY_PRIVATE, got.Object.Visibility)
	require.Equal(t, pb.MimeType_MIME_TYPE_PDF, got.Object.MimeType)
	require.Equal(t, int64(123), got.Object.SizeBytes)
	require.Equal(t, int64(100), got.Object.CreatedAtUnix)
	require.True(t, got.Indexed)
	require.Equal(t, int32(4), got.GetChunksUpserted())
}

func TestGetAndDeleteFileRequestMappings(t *testing.T) {
	get := GetFileRequestToEntity(&pb.GetFileRequest{UserId: testUserIDForUtil, ObjectId: "object-1"})
	require.Equal(t, entity.GetFileRequest{CallerID: testUserIDForUtil, ObjectID: "object-1"}, get)

	del := DeleteFileRequestToEntity(&pb.DeleteFileRequest{UserId: testUserIDForUtil, ObjectId: "object-1"})
	require.Equal(t, entity.DeleteFileRequest{CallerID: testUserIDForUtil, ObjectID: "object-1"}, del)
}

func TestGetAndDeleteFileResponseMappings(t *testing.T) {
	now := time.Unix(200, 0)
	get := GetFileResponseToProto(entity.GetFileResponse{
		Object:  entity.ObjectMetadata{ID: "object-1", CreatedAt: now, UpdatedAt: now},
		Content: []byte("raw-bytes"),
	})
	require.Equal(t, []byte("raw-bytes"), get.Content)
	require.Equal(t, "object-1", get.Object.ObjectId)

	del := DeleteFileResponseToProto(entity.DeleteFileResponse{Deleted: true})
	require.True(t, del.Deleted)
}

func TestObjectTypeMappings(t *testing.T) {
	require.Equal(t, entity.ObjectTypeProfilePicture, ObjectTypeFromProto(pb.ObjectType_OBJECT_TYPE_PROFILE_PICTURE))
	require.Equal(t, entity.ObjectTypeResourceDocument, ObjectTypeFromProto(pb.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT))
	require.Empty(t, ObjectTypeFromProto(pb.ObjectType_OBJECT_TYPE_UNSPECIFIED))
	require.Equal(t, pb.ObjectType_OBJECT_TYPE_PROFILE_PICTURE, ObjectTypeToProto(entity.ObjectTypeProfilePicture))
	require.Equal(t, pb.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT, ObjectTypeToProto(entity.ObjectTypeResourceDocument))
	require.Equal(t, pb.ObjectType_OBJECT_TYPE_UNSPECIFIED, ObjectTypeToProto("unknown"))
}

func TestVisibilityMappings(t *testing.T) {
	require.Equal(t, entity.ObjectVisibilityPublic, ObjectVisibilityFromProto(pb.ObjectVisibility_OBJECT_VISIBILITY_PUBLIC))
	require.Equal(t, entity.ObjectVisibilityPrivate, ObjectVisibilityFromProto(pb.ObjectVisibility_OBJECT_VISIBILITY_PRIVATE))
	require.Empty(t, ObjectVisibilityFromProto(pb.ObjectVisibility_OBJECT_VISIBILITY_UNSPECIFIED))
	require.Equal(t, pb.ObjectVisibility_OBJECT_VISIBILITY_PUBLIC, ObjectVisibilityToProto(entity.ObjectVisibilityPublic))
	require.Equal(t, pb.ObjectVisibility_OBJECT_VISIBILITY_PRIVATE, ObjectVisibilityToProto(entity.ObjectVisibilityPrivate))
	require.Equal(t, pb.ObjectVisibility_OBJECT_VISIBILITY_UNSPECIFIED, ObjectVisibilityToProto("unknown"))
}

func TestMIMETypeMappings(t *testing.T) {
	require.Equal(t, entity.MIMETypePDF, MimeTypeFromProto(pb.MimeType_MIME_TYPE_PDF))
	require.Equal(t, entity.MIMETypeJPEG, MimeTypeFromProto(pb.MimeType_MIME_TYPE_JPEG))
	require.Equal(t, entity.MIMETypePNG, MimeTypeFromProto(pb.MimeType_MIME_TYPE_PNG))
	require.Equal(t, entity.MIMETypeWEBP, MimeTypeFromProto(pb.MimeType_MIME_TYPE_WEBP))
	require.Empty(t, MimeTypeFromProto(pb.MimeType_MIME_TYPE_UNSPECIFIED))
	require.Equal(t, pb.MimeType_MIME_TYPE_PDF, MimeTypeToProto(entity.MIMETypePDF))
	require.Equal(t, pb.MimeType_MIME_TYPE_JPEG, MimeTypeToProto(entity.MIMETypeJPEG))
	require.Equal(t, pb.MimeType_MIME_TYPE_PNG, MimeTypeToProto(entity.MIMETypePNG))
	require.Equal(t, pb.MimeType_MIME_TYPE_WEBP, MimeTypeToProto(entity.MIMETypeWEBP))
	require.Equal(t, pb.MimeType_MIME_TYPE_UNSPECIFIED, MimeTypeToProto("unknown"))
}

func TestUploadFileRequestToEntity_NilRequest(t *testing.T) {
	_, err := UploadFileRequestToEntity(nil)
	require.Error(t, err)
}

func TestListFilesRequestToEntity(t *testing.T) {
	req := &pb.ListFilesRequest{
		UserId:     testUserIDForUtil,
		ObjectType: pb.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT,
	}

	got := ListFilesRequestToEntity(req)

	require.Equal(t, entity.ListFilesRequest{
		CallerID:   testUserIDForUtil,
		ObjectType: entity.ObjectTypeResourceDocument,
	}, got)
}

func TestListFilesResponseToProto(t *testing.T) {
	createdAt := time.Unix(100, 0)
	updatedAt := time.Unix(200, 0)

	response := entity.ListFilesResponse{
		Objects: []entity.ObjectMetadata{
			{
				ID:               "object-1",
				UserID:           testUserIDForUtil,
				ObjectType:       entity.ObjectTypeResourceDocument,
				Visibility:       entity.ObjectVisibilityPrivate,
				OriginalFileName: "resume.pdf",
				MIMEType:         entity.MIMETypePDF,
				SizeBytes:        12345,
				CreatedAt:        createdAt,
				UpdatedAt:        updatedAt,
			},
		},
	}

	got := ListFilesResponseToProto(response)

	require.Len(t, got.Objects, 1)

	object := got.Objects[0]

	require.Equal(t, "object-1", object.ObjectId)
	require.Equal(t, testUserIDForUtil, object.OwnerUserId)
	require.Equal(t, pb.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT, object.ObjectType)
	require.Equal(t, pb.ObjectVisibility_OBJECT_VISIBILITY_PRIVATE, object.Visibility)
	require.Equal(t, "resume.pdf", object.OriginalFileName)
	require.Equal(t, pb.MimeType_MIME_TYPE_PDF, object.MimeType)
	require.Equal(t, int64(12345), object.SizeBytes)
	require.Equal(t, int64(100), object.CreatedAtUnix)
	require.Equal(t, int64(200), object.UpdatedAtUnix)
}

func TestListFilesResponseToProto_Empty(t *testing.T) {
	got := ListFilesResponseToProto(entity.ListFilesResponse{})

	require.NotNil(t, got)
	require.Empty(t, got.Objects)
}
