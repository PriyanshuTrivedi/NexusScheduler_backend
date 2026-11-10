package util

import (
	"fmt"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	chatbotpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
)

func ValidateIndexDocumentRequest(req *entity.IndexDocumentRequest) error {
	if req == nil {
		return fmt.Errorf("index document request is nil")
	}

	if req.ObjectID == "" {
		return fmt.Errorf("object ID is required")
	}

	if req.OwnerUserID == "" {
		return fmt.Errorf("owner user ID is required")
	}

	if req.IdempotencyKey == "" {
		return fmt.Errorf("idempotency key is required")
	}

	if req.DocumentType != entity.DocumentTypePDF {
		return fmt.Errorf("unsupported document type %q", req.DocumentType)
	}

	if len(req.Content) == 0 {
		return fmt.Errorf("document content is empty")
	}

	return nil
}

func ValidateDeleteDocumentIndexRequest(req *entity.DeleteDocumentIndexRequest) error {
	if req == nil {
		return fmt.Errorf("delete document index request is nil")
	}

	if req.ObjectID == "" {
		return fmt.Errorf("object ID is required")
	}

	if req.OwnerUserID == "" {
		return fmt.Errorf("owner user ID is required")
	}

	return nil
}

func IndexDocumentRequestFromProto(req *chatbotpb.IndexDocumentRequest) *entity.IndexDocumentRequest {
	if req == nil {
		return nil
	}

	documentType := entity.DocumentTypePDF
	if req.DocumentType != chatbotpb.DocumentType_DOCUMENT_TYPE_PDF {
		documentType = entity.DocumentType(req.DocumentType.String())
	}

	return &entity.IndexDocumentRequest{
		ObjectID:       req.ObjectId,
		OwnerUserID:    req.OwnerUserId,
		IdempotencyKey: req.IdempotencyKey,
		DocumentType:   documentType,
		Content:        req.Content,
	}
}

func IndexDocumentResponseToProto(resp *entity.IndexDocumentResponse) *chatbotpb.IndexDocumentResponse {
	if resp == nil {
		return nil
	}

	return &chatbotpb.IndexDocumentResponse{
		Indexed:        resp.Indexed,
		ChunksUpserted: resp.ChunksUpserted,
	}
}

func DeleteDocumentIndexRequestFromProto(req *chatbotpb.DeleteDocumentIndexRequest) *entity.DeleteDocumentIndexRequest {
	if req == nil {
		return nil
	}

	return &entity.DeleteDocumentIndexRequest{
		ObjectID:    req.ObjectId,
		OwnerUserID: req.OwnerUserId,
	}
}

func DeleteDocumentIndexResponseToProto(resp *entity.DeleteDocumentIndexResponse) *chatbotpb.DeleteDocumentIndexResponse {
	if resp == nil {
		return nil
	}

	return &chatbotpb.DeleteDocumentIndexResponse{
		Deleted: resp.Deleted,
	}
}
