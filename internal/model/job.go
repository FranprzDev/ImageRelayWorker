package model

type ImageJob struct {
	ID        string `json:"id"`
	ImageURL  string `json:"imageUrl"`
	ProductID string `json:"productId"`
}
type ClaimRequest struct {
	WorkerID string `json:"workerId"`
}
type ClaimResponse struct {
	Job *ImageJob `json:"job"`
}
type CompleteRequest struct {
	WorkerID    string `json:"workerId"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"contentType"`
	Bytes       int64  `json:"bytes"`
}
type FailRequest struct {
	WorkerID string `json:"workerId"`
	Error    string `json:"error"`
}

type JobStatus struct {
	ID     string      `json:"id"`
	State  string      `json:"state"`
	Upload *UploadInfo `json:"upload"`
}

type UploadInfo struct {
	Bytes       int64  `json:"bytes"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"contentType"`
}
