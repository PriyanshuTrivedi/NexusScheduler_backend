package entity

type DocumentType string

const (
	DocumentTypePDF DocumentType = "pdf"
)

type IndexDocumentRequest struct {
	ObjectID       string
	OwnerUserID    string
	IdempotencyKey string
	DocumentType   DocumentType
	Content        []byte
}

type IndexDocumentResponse struct {
	Indexed        bool
	ChunksUpserted int32
}

type DeleteDocumentIndexRequest struct {
	ObjectID    string
	OwnerUserID string
}

type DeleteDocumentIndexResponse struct {
	Deleted bool
}
