package util

import (
	"fmt"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
)

func UploadFileRequestToEntity(req *identitypb.UploadFileRequest) (entity.UploadFileRequest, error) {
	if req == nil {
		return entity.UploadFileRequest{}, fmt.Errorf("identity: upload file request is nil")
	}

	return entity.UploadFileRequest{
		UserID:         req.GetUserId(),
		FileName:       req.GetFileName(),
		MIMEType:       MimeTypeFromProto(req.GetMimeType()),
		ObjectType:     ObjectTypeFromProto(req.GetObjectType()),
		Visibility:     ObjectVisibilityFromProto(req.GetVisibility()),
		Content:        req.GetContent(),
		IdempotencyKey: req.GetIdempotencyKey(),
	}, nil
}

func UploadFileResponseToProto(response entity.UploadFileResponse) *identitypb.UploadFileResponse {
	return &identitypb.UploadFileResponse{
		Object:         ObjectMetadataToProto(response.Object),
		Indexed:        response.Indexed,
		ChunksUpserted: response.ChunksUpserted,
	}
}

func GetFileRequestToEntity(req *identitypb.GetFileRequest) entity.GetFileRequest {
	return entity.GetFileRequest{
		CallerID: req.GetUserId(),
		ObjectID: req.GetObjectId(),
	}
}

func GetFileResponseToProto(response entity.GetFileResponse) *identitypb.GetFileResponse {
	return &identitypb.GetFileResponse{
		Object:  ObjectMetadataToProto(response.Object),
		Content: response.Content,
	}
}

func DeleteFileRequestToEntity(req *identitypb.DeleteFileRequest) entity.DeleteFileRequest {
	return entity.DeleteFileRequest{
		CallerID: req.GetUserId(),
		ObjectID: req.GetObjectId(),
	}
}

func DeleteFileResponseToProto(response entity.DeleteFileResponse) *identitypb.DeleteFileResponse {
	return &identitypb.DeleteFileResponse{
		Deleted: response.Deleted,
	}
}

func ObjectMetadataToProto(metadata entity.ObjectMetadata) *identitypb.ObjectMetadata {
	return &identitypb.ObjectMetadata{
		ObjectId:         metadata.ID,
		OwnerUserId:      metadata.UserID,
		ObjectType:       ObjectTypeToProto(metadata.ObjectType),
		Visibility:       ObjectVisibilityToProto(metadata.Visibility),
		OriginalFileName: metadata.OriginalFileName,
		MimeType:         MimeTypeToProto(metadata.MIMEType),
		SizeBytes:        metadata.SizeBytes,
		CreatedAtUnix:    metadata.CreatedAt.Unix(),
		UpdatedAtUnix:    metadata.UpdatedAt.Unix(),
	}
}

func ObjectTypeFromProto(objectType identitypb.ObjectType) string {
	switch objectType {
	case identitypb.ObjectType_OBJECT_TYPE_PROFILE_PICTURE:
		return entity.ObjectTypeProfilePicture
	case identitypb.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT:
		return entity.ObjectTypeResourceDocument
	default:
		return ""
	}
}

func ObjectTypeToProto(objectType string) identitypb.ObjectType {
	switch objectType {
	case entity.ObjectTypeProfilePicture:
		return identitypb.ObjectType_OBJECT_TYPE_PROFILE_PICTURE
	case entity.ObjectTypeResourceDocument:
		return identitypb.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT
	default:
		return identitypb.ObjectType_OBJECT_TYPE_UNSPECIFIED
	}
}

func ObjectVisibilityFromProto(visibility identitypb.ObjectVisibility) string {
	switch visibility {
	case identitypb.ObjectVisibility_OBJECT_VISIBILITY_PUBLIC:
		return entity.ObjectVisibilityPublic
	case identitypb.ObjectVisibility_OBJECT_VISIBILITY_PRIVATE:
		return entity.ObjectVisibilityPrivate
	default:
		return ""
	}
}

func ObjectVisibilityToProto(visibility string) identitypb.ObjectVisibility {
	switch visibility {
	case entity.ObjectVisibilityPublic:
		return identitypb.ObjectVisibility_OBJECT_VISIBILITY_PUBLIC
	case entity.ObjectVisibilityPrivate:
		return identitypb.ObjectVisibility_OBJECT_VISIBILITY_PRIVATE
	default:
		return identitypb.ObjectVisibility_OBJECT_VISIBILITY_UNSPECIFIED
	}
}

func MimeTypeFromProto(mimeType identitypb.MimeType) string {
	switch mimeType {
	case identitypb.MimeType_MIME_TYPE_PDF:
		return entity.MIMETypePDF
	case identitypb.MimeType_MIME_TYPE_JPEG:
		return entity.MIMETypeJPEG
	case identitypb.MimeType_MIME_TYPE_PNG:
		return entity.MIMETypePNG
	case identitypb.MimeType_MIME_TYPE_WEBP:
		return entity.MIMETypeWEBP
	default:
		return ""
	}
}

func MimeTypeToProto(mimeType string) identitypb.MimeType {
	switch mimeType {
	case entity.MIMETypePDF:
		return identitypb.MimeType_MIME_TYPE_PDF
	case entity.MIMETypeJPEG:
		return identitypb.MimeType_MIME_TYPE_JPEG
	case entity.MIMETypePNG:
		return identitypb.MimeType_MIME_TYPE_PNG
	case entity.MIMETypeWEBP:
		return identitypb.MimeType_MIME_TYPE_WEBP
	default:
		return identitypb.MimeType_MIME_TYPE_UNSPECIFIED
	}
}
