package entity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	validObjectUserID         = "11111111-1111-4111-8111-111111111111"
	validObjectIdempotencyKey = "44444444-4444-4444-8444-444444444444"
)

func validObjectUploadRequest() UploadFileRequest {
	return UploadFileRequest{
		UserID: validObjectUserID, FileName: "resume.pdf", MIMEType: MIMETypePDF,
		ObjectType: ObjectTypeResourceDocument, Visibility: ObjectVisibilityPrivate,
		Content: []byte("%PDF-test"),
	}
}

func TestUploadFileRequest_NormalizeAndValidate_ResourceDocument(t *testing.T) {
	req := validObjectUploadRequest()
	req.UserID = " " + req.UserID + " "
	req.FileName = " resume.pdf "
	req.ObjectType = " resource_document "
	req.Visibility = " private "

	require.NoError(t, req.NormalizeAndValidate())
	require.Equal(t, validObjectUserID, req.UserID)
	require.Equal(t, "resume.pdf", req.FileName)
	require.Equal(t, ObjectTypeResourceDocument, req.ObjectType)
	require.Equal(t, ObjectVisibilityPrivate, req.Visibility)
}

func TestUploadFileRequest_NormalizeAndValidate_ProfilePicture(t *testing.T) {
	for _, mimeType := range []string{MIMETypeJPEG, MIMETypePNG, MIMETypeWEBP} {
		t.Run(mimeType, func(t *testing.T) {
			req := UploadFileRequest{
				UserID: validObjectUserID, FileName: "avatar", MIMEType: mimeType,
				ObjectType: ObjectTypeProfilePicture, Visibility: ObjectVisibilityPublic,
				Content: []byte("image"),
			}
			require.NoError(t, req.NormalizeAndValidate())
		})
	}
}

func TestUploadFileRequest_InvalidBaseFields(t *testing.T) {
	tests := []struct {
		name string
		edit func(*UploadFileRequest)
	}{
		{"invalid user id", func(r *UploadFileRequest) { r.UserID = "not-a-uuid" }},
		{"empty file name", func(r *UploadFileRequest) { r.FileName = " " }},
		{"empty mime type", func(r *UploadFileRequest) { r.MIMEType = "" }},
		{"empty content", func(r *UploadFileRequest) { r.Content = nil }},
		{"invalid idempotency key", func(r *UploadFileRequest) { r.IdempotencyKey = "not-a-uuid" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validObjectUploadRequest()
			tt.edit(&req)
			require.ErrorIs(t, req.NormalizeAndValidate(), ErrInvalidFile)
		})
	}
}

func TestUploadFileRequest_InvalidObjectType(t *testing.T) {
	req := validObjectUploadRequest()
	req.ObjectType = "UNKNOWN"
	require.ErrorIs(t, req.NormalizeAndValidate(), ErrInvalidObjectType)
}

func TestUploadFileRequest_InvalidVisibility(t *testing.T) {
	req := validObjectUploadRequest()
	req.Visibility = "UNKNOWN"
	require.ErrorIs(t, req.NormalizeAndValidate(), ErrInvalidVisibility)
}

func TestUploadFileRequest_ProfilePictureRequiresPublicImage(t *testing.T) {
	req := validObjectUploadRequest()
	req.ObjectType = ObjectTypeProfilePicture
	req.MIMEType = MIMETypePNG
	req.Visibility = ObjectVisibilityPrivate
	require.ErrorIs(t, req.NormalizeAndValidate(), ErrInvalidVisibility)

	req.Visibility = ObjectVisibilityPublic
	req.MIMEType = MIMETypePDF
	require.ErrorIs(t, req.NormalizeAndValidate(), ErrInvalidFile)
}

func TestUploadFileRequest_ResourceDocumentRequiresPrivatePDFWithinLimit(t *testing.T) {
	req := validObjectUploadRequest()
	req.Visibility = ObjectVisibilityPublic
	require.ErrorIs(t, req.NormalizeAndValidate(), ErrInvalidVisibility)

	req = validObjectUploadRequest()
	req.MIMEType = MIMETypePNG
	require.ErrorIs(t, req.NormalizeAndValidate(), ErrInvalidFile)

	req = validObjectUploadRequest()
	req.Content = make([]byte, MaxPDFSizeBytes+1)
	require.ErrorIs(t, req.NormalizeAndValidate(), ErrInvalidFile)
}

func TestContentSHA256(t *testing.T) {
	require.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", ContentSHA256([]byte("hello")))
}

func TestIsUUID(t *testing.T) {
	require.True(t, IsUUID(validObjectUserID))
	require.True(t, IsUUID("AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"))
	require.False(t, IsUUID("not-a-uuid"))
	require.False(t, IsUUID(""))
}
