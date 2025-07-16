package upload_request

import (
	p_registered_service "github.com/file_upload_microservice/registered_service"
)

type UploadRequest struct {
	UploadID    string
	Service     *p_registered_service.Service
	ChunkSize   int
	TotalChunks int
	FileName    string
	ParentPath  string
	// in the future maybe add information about the main service

}
