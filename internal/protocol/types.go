package protocol

import "time"

type HelloRequest struct {
	Ver         int    `json:"ver"`
	Auth        string `json:"auth"`
	JobID       int64  `json:"job_id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	Compression string `json:"compression"`
	Encryption  string `json:"encryption"`
}

type FinalResponse struct {
	JobID      int64     `json:"job_id"`
	Status     string    `json:"status"`
	Reason     string    `json:"reason,omitempty"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256"`
	ReceivedAt time.Time `json:"received_at"`
	StoredPath string    `json:"stored_path"`
}
