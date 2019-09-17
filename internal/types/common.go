package types

type StartUploadRequest struct {
	FileName string `json:"fileName"`
	FileType string `json:"fileType"`
}

type StartUploadResponse struct {
	UploadID string `json:"uploadId"`
}

type GetUploadURLRequest struct {
	FileName   string `json:"fileName"`
	PartNumber string `json:"partNumber"`
	UploadID   string `json:"uploadId"`
}

type GetUploadURLResponse struct {
	PresignedURL string `json:"presignedUrl"`
}

type CompleteUploadRequest struct {
	Params CompleteUploadParams `json:"params"`
}

type CompleteUploadParams struct {
	FileName string               `json:"fileName"`
	Parts    []CompleteUploadPart `json:"parts"`
	UploadID string               `json:"uploadId"`
}

type CompleteUploadPart struct {
	ETag       string
	PartNumber int64
}

type CompleteUploadResponse struct {
	Data CompleteUploadData `json:"data"`
}

type CompleteUploadData struct {
	Location string
	Bucket   string
	Key      string
	ETag     string
}
