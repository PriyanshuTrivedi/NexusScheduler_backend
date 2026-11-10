package util

import (
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/chatbot/entity"
	chatbotpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
	"github.com/stretchr/testify/require"
)

func validIndexDocumentRequest() *entity.IndexDocumentRequest {
	return &entity.IndexDocumentRequest{
		ObjectID:       "object-1",
		OwnerUserID:    "user-1",
		IdempotencyKey: "key-1",
		DocumentType:   entity.DocumentTypePDF,
		Content:        []byte("%PDF-1.4"),
	}
}

func TestValidateIndexDocumentRequest(t *testing.T) {
	tests := []struct {
		name    string
		request *entity.IndexDocumentRequest
		wantErr string
	}{
		{name: "valid request", request: validIndexDocumentRequest()},
		{name: "nil request", wantErr: "index document request is nil"},
		{name: "missing object ID", request: func() *entity.IndexDocumentRequest { r := validIndexDocumentRequest(); r.ObjectID = ""; return r }(), wantErr: "object ID is required"},
		{name: "missing owner user ID", request: func() *entity.IndexDocumentRequest { r := validIndexDocumentRequest(); r.OwnerUserID = ""; return r }(), wantErr: "owner user ID is required"},
		{name: "missing idempotency key", request: func() *entity.IndexDocumentRequest { r := validIndexDocumentRequest(); r.IdempotencyKey = ""; return r }(), wantErr: "idempotency key is required"},
		{name: "unsupported document type", request: func() *entity.IndexDocumentRequest {
			r := validIndexDocumentRequest()
			r.DocumentType = "docx"
			return r
		}(), wantErr: `unsupported document type "docx"`},
		{name: "empty content", request: func() *entity.IndexDocumentRequest { r := validIndexDocumentRequest(); r.Content = nil; return r }(), wantErr: "document content is empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIndexDocumentRequest(tt.request)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestValidateDeleteDocumentIndexRequest(t *testing.T) {
	tests := []struct {
		name    string
		request *entity.DeleteDocumentIndexRequest
		wantErr string
	}{
		{name: "valid request", request: &entity.DeleteDocumentIndexRequest{ObjectID: "object-1", OwnerUserID: "user-1"}},
		{name: "nil request", wantErr: "delete document index request is nil"},
		{name: "missing object ID", request: &entity.DeleteDocumentIndexRequest{OwnerUserID: "user-1"}, wantErr: "object ID is required"},
		{name: "missing owner user ID", request: &entity.DeleteDocumentIndexRequest{ObjectID: "object-1"}, wantErr: "owner user ID is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeleteDocumentIndexRequest(tt.request)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestIndexDocumentRequestFromProto(t *testing.T) {
	require.Nil(t, IndexDocumentRequestFromProto(nil))

	content := []byte("%PDF-1.4")
	got := IndexDocumentRequestFromProto(&chatbotpb.IndexDocumentRequest{
		ObjectId:       "object-1",
		OwnerUserId:    "user-1",
		IdempotencyKey: "key-1",
		DocumentType:   chatbotpb.DocumentType_DOCUMENT_TYPE_PDF,
		Content:        content,
	})
	require.Equal(t, &entity.IndexDocumentRequest{
		ObjectID:       "object-1",
		OwnerUserID:    "user-1",
		IdempotencyKey: "key-1",
		DocumentType:   entity.DocumentTypePDF,
		Content:        content,
	}, got)
}

func TestIndexDocumentRequestFromProto_UnknownDocumentType(t *testing.T) {
	got := IndexDocumentRequestFromProto(&chatbotpb.IndexDocumentRequest{
		DocumentType: chatbotpb.DocumentType(99),
	})
	require.Equal(t, entity.DocumentType("99"), got.DocumentType)
	require.Error(t, ValidateIndexDocumentRequest(got))
}

func TestIndexDocumentResponseToProto(t *testing.T) {
	require.Nil(t, IndexDocumentResponseToProto(nil))
	got := IndexDocumentResponseToProto(&entity.IndexDocumentResponse{Indexed: true, ChunksUpserted: 7})
	require.Equal(t, &chatbotpb.IndexDocumentResponse{Indexed: true, ChunksUpserted: 7}, got)
}

func TestDeleteDocumentIndexRequestFromProto(t *testing.T) {
	require.Nil(t, DeleteDocumentIndexRequestFromProto(nil))
	got := DeleteDocumentIndexRequestFromProto(&chatbotpb.DeleteDocumentIndexRequest{
		ObjectId: "object-1", OwnerUserId: "user-1",
	})
	require.Equal(t, &entity.DeleteDocumentIndexRequest{ObjectID: "object-1", OwnerUserID: "user-1"}, got)
}

func TestDeleteDocumentIndexResponseToProto(t *testing.T) {
	require.Nil(t, DeleteDocumentIndexResponseToProto(nil))
	got := DeleteDocumentIndexResponseToProto(&entity.DeleteDocumentIndexResponse{Deleted: true})
	require.Equal(t, &chatbotpb.DeleteDocumentIndexResponse{Deleted: true}, got)
}
