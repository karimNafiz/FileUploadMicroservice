package upload_session

import (
	"net"
	"time"

	p_chunk_job "github.com/file_upload_microservice/chunk_job"
)

// for this upload session I need to have
// channel chunk request
// channel chunk error
// channel chunk acks
// channel done (not sure about this one)
type UploadSession struct {
	UploadID                      string
	TotalChunks                   int
	ChunksUploaded                int
	ChunksUploadedSinceLastUpdate int
	ChunkSize                     int
	ParentPath                    string
	FileName                      string
	LastActivity                  time.Time

	// important for session state
	// i can think of better names later
	Conn net.Conn
	In   chan *p_chunk_job.ChunkJob
	Err  chan *p_chunk_job.ChunkJobError
	Acks chan *p_chunk_job.ChunkJobAck
	Done chan struct{}
}

// need a function to notify a chunk has been uploaded
// need a function to reset

// still in intial state do not know what this function should return
// I will have to consider using locks
// because the same entry could be accessed by multiple go routine
func (u *UploadSession) NotifyConfirmation() {
	// keeping this function very simple right now
	u.ChunksUploaded++
	u.ChunksUploadedSinceLastUpdate++
}
func NewUploadSession(conn net.Conn, uploadID string, totalChunks int, chunkSize int, parentPath, fileName string) *UploadSession {
	return &UploadSession{
		Conn:         conn,
		UploadID:     uploadID,
		TotalChunks:  totalChunks,
		ChunkSize:    chunkSize,
		ParentPath:   parentPath,
		FileName:     fileName,
		LastActivity: time.Now(),
	}
}
