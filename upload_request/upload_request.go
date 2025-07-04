package upload_request

type UploadRequest struct {
	UploadID    string
	ChunkSize   int
	TotalChunks int
	FileName    string
	ParentPath  string
	// in the future maybe add information about the main service

}
