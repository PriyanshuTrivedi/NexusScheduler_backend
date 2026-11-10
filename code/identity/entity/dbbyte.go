package entity

type DBByteFile struct {
	Success  bool   `json:"success"`
	Key      string `json:"key"`
	FileID   string `json:"fileId"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MIMEType string `json:"mimeType"`
}
