package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	controllermocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/identity/controller"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*Handler, *controllermocks.MockController, *controllermocks.MockObjectStorageController) {
	t.Helper()
	ctrl := gomock.NewController(t)
	userController := controllermocks.NewMockController(ctrl)
	objectStorageController := controllermocks.NewMockObjectStorageController(ctrl)
	return New(userController, objectStorageController), userController, objectStorageController
}

func TestRegisterClient(t *testing.T) {
	h, userController, _ := newTestHandler(t)
	userController.EXPECT().
		RegisterClient(gomock.Any(), "Alice", entity.UserIdentifier{Email: "a@example.com"}, "pw").
		Return(entity.User{ID: "u-1", Role: entity.RoleClient}, nil)

	resp, err := h.RegisterClient(context.Background(), &identity.RegisterClientRequest{
		Name:       "Alice",
		Identifier: &identity.UserIdentifier{Value: &identity.UserIdentifier_Email{Email: "a@example.com"}},
		Password:   "pw",
	})
	require.NoError(t, err)
	require.Equal(t, "u-1", resp.User.UserId)
}

func TestRegisterProvider(t *testing.T) {
	h, userController, _ := newTestHandler(t)
	userController.EXPECT().
		RegisterProvider(gomock.Any(), "Dr. Mehta", entity.UserIdentifier{Email: "doctor@example.com"}, "pw", entity.TenantTypeIndividual, "").
		Return(entity.User{ID: "r-1", Role: entity.RoleResource}, nil)

	resp, err := h.RegisterProvider(context.Background(), &identity.RegisterProviderRequest{
		Name:       "Dr. Mehta",
		Identifier: &identity.UserIdentifier{Value: &identity.UserIdentifier_Email{Email: "doctor@example.com"}},
		Password:   "pw",
		TenantType: identity.TenantType_TENANT_TYPE_INDIVIDUAL,
	})
	require.NoError(t, err)
	require.Equal(t, "r-1", resp.User.UserId)
}

func TestUpdateProfilePassesRequestedUserID(t *testing.T) {
	h, userController, _ := newTestHandler(t)
	userController.EXPECT().
		UpdateProfile(gomock.Any(), "u-1", "Alice").
		Return(entity.User{ID: "u-1", Name: "Alice", Role: entity.RoleClient}, nil)

	_, err := h.UpdateProfile(context.Background(), &identity.UpdateProfileRequest{UserId: "u-1", Name: "Alice"})
	require.NoError(t, err)
}

func TestGetUserDoesNotPerformAuthorization(t *testing.T) {
	h, userController, _ := newTestHandler(t)
	userController.EXPECT().
		GetUser(gomock.Any(), "u-2").
		Return(entity.User{ID: "u-2", Role: entity.RoleClient}, nil)

	resp, err := h.GetUser(context.Background(), &identity.GetUserRequest{UserId: "u-2"})
	require.NoError(t, err)
	require.Equal(t, "u-2", resp.User.UserId)
}

func TestUploadFile(t *testing.T) {
	h, _, objectController := newTestHandler(t)
	request := &identity.UploadFileRequest{
		UserId:   "11111111-1111-4111-8111-111111111111",
		FileName: "avatar.png", MimeType: identity.MimeType_MIME_TYPE_PNG,
		ObjectType: identity.ObjectType_OBJECT_TYPE_PROFILE_PICTURE,
		Visibility: identity.ObjectVisibility_OBJECT_VISIBILITY_PUBLIC,
		Content:    []byte("image"),
	}
	objectController.EXPECT().
		UploadFile(gomock.Any(), entity.UploadFileRequest{
			UserID: request.UserId, FileName: request.FileName, MIMEType: entity.MIMETypePNG,
			ObjectType: entity.ObjectTypeProfilePicture, Visibility: entity.ObjectVisibilityPublic,
			Content: []byte("image"),
		}).
		Return(entity.UploadFileResponse{
			Object: entity.ObjectMetadata{ID: "object-1", UserID: request.UserId, OriginalFileName: "avatar.png"},
		}, nil)

	resp, err := h.UploadFile(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, "object-1", resp.Object.ObjectId)
}

func TestUploadFile_ControllerError(t *testing.T) {
	h, _, objectController := newTestHandler(t)
	controllerErr := errors.New("upload failed")
	objectController.EXPECT().UploadFile(gomock.Any(), gomock.Any()).
		Return(entity.UploadFileResponse{}, controllerErr)

	_, err := h.UploadFile(context.Background(), &identity.UploadFileRequest{
		UserId:   "11111111-1111-4111-8111-111111111111",
		FileName: "avatar.png", MimeType: identity.MimeType_MIME_TYPE_PNG,
		ObjectType: identity.ObjectType_OBJECT_TYPE_PROFILE_PICTURE,
		Visibility: identity.ObjectVisibility_OBJECT_VISIBILITY_PUBLIC, Content: []byte("image"),
	})
	require.ErrorIs(t, err, controllerErr)
}

func TestGetFile(t *testing.T) {
	h, _, objectController := newTestHandler(t)
	objectController.EXPECT().
		GetFile(gomock.Any(), entity.GetFileRequest{
			CallerID: "11111111-1111-4111-8111-111111111111", ObjectID: "33333333-3333-4333-8333-333333333333",
		}).
		Return(entity.GetFileResponse{
			Object:  entity.ObjectMetadata{ID: "33333333-3333-4333-8333-333333333333"},
			Content: []byte("raw-content"),
		}, nil)

	resp, err := h.GetFile(context.Background(), &identity.GetFileRequest{
		UserId: "11111111-1111-4111-8111-111111111111", ObjectId: "33333333-3333-4333-8333-333333333333",
	})
	require.NoError(t, err)
	require.Equal(t, []byte("raw-content"), resp.Content)
	require.Equal(t, "33333333-3333-4333-8333-333333333333", resp.Object.ObjectId)
}

func TestDeleteFile(t *testing.T) {
	h, _, objectController := newTestHandler(t)
	objectController.EXPECT().
		DeleteFile(gomock.Any(), entity.DeleteFileRequest{
			CallerID: "11111111-1111-4111-8111-111111111111", ObjectID: "33333333-3333-4333-8333-333333333333",
		}).
		Return(entity.DeleteFileResponse{Deleted: true}, nil)

	resp, err := h.DeleteFile(context.Background(), &identity.DeleteFileRequest{
		UserId: "11111111-1111-4111-8111-111111111111", ObjectId: "33333333-3333-4333-8333-333333333333",
	})
	require.NoError(t, err)
	require.True(t, resp.Deleted)
}

func TestListFiles(t *testing.T) {
	h, _, objectController := newTestHandler(t)

	userID := "11111111-1111-4111-8111-111111111111"
	objectID := "33333333-3333-4333-8333-333333333333"
	createdAt := time.Unix(100, 0)
	updatedAt := time.Unix(200, 0)

	objectController.EXPECT().
		ListFiles(gomock.Any(), entity.ListFilesRequest{
			CallerID:   userID,
			ObjectType: entity.ObjectTypeResourceDocument,
		}).
		Return(entity.ListFilesResponse{
			Objects: []entity.ObjectMetadata{
				{
					ID:               objectID,
					UserID:           userID,
					ObjectType:       entity.ObjectTypeResourceDocument,
					Visibility:       entity.ObjectVisibilityPrivate,
					OriginalFileName: "resume.pdf",
					MIMEType:         entity.MIMETypePDF,
					SizeBytes:        12345,
					CreatedAt:        createdAt,
					UpdatedAt:        updatedAt,
				},
			},
		}, nil)

	resp, err := h.ListFiles(context.Background(), &identity.ListFilesRequest{
		UserId:     userID,
		ObjectType: identity.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Objects, 1)

	object := resp.Objects[0]
	require.Equal(t, objectID, object.ObjectId)
	require.Equal(t, userID, object.OwnerUserId)
	require.Equal(t, identity.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT, object.ObjectType)
	require.Equal(t, identity.ObjectVisibility_OBJECT_VISIBILITY_PRIVATE, object.Visibility)
	require.Equal(t, "resume.pdf", object.OriginalFileName)
	require.Equal(t, identity.MimeType_MIME_TYPE_PDF, object.MimeType)
	require.Equal(t, int64(12345), object.SizeBytes)
	require.Equal(t, int64(100), object.CreatedAtUnix)
	require.Equal(t, int64(200), object.UpdatedAtUnix)
}

func TestListFiles_EmptyList(t *testing.T) {
	h, _, objectController := newTestHandler(t)

	userID := "11111111-1111-4111-8111-111111111111"

	objectController.EXPECT().
		ListFiles(gomock.Any(), entity.ListFilesRequest{
			CallerID:   userID,
			ObjectType: entity.ObjectTypeResourceDocument,
		}).
		Return(entity.ListFilesResponse{
			Objects: []entity.ObjectMetadata{},
		}, nil)

	resp, err := h.ListFiles(context.Background(), &identity.ListFilesRequest{
		UserId:     userID,
		ObjectType: identity.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Empty(t, resp.Objects)
}

func TestListFiles_ControllerError(t *testing.T) {
	h, _, objectController := newTestHandler(t)

	userID := "11111111-1111-4111-8111-111111111111"
	controllerErr := errors.New("list files failed")

	objectController.EXPECT().
		ListFiles(gomock.Any(), entity.ListFilesRequest{
			CallerID:   userID,
			ObjectType: entity.ObjectTypeResourceDocument,
		}).
		Return(entity.ListFilesResponse{}, controllerErr)

	resp, err := h.ListFiles(context.Background(), &identity.ListFilesRequest{
		UserId:     userID,
		ObjectType: identity.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT,
	})

	require.ErrorIs(t, err, controllerErr)
	require.Nil(t, resp)
}
