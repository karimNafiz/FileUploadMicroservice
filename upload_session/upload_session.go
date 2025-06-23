package upload_session

import (
	"net"
	"time"
)

type UploadSessionState struct {
	Conn                          net.Conn
	UploadID                      string
	TotalChunks                   int
	ChunksUploaded                int
	ChunksUploadedSinceLastUpdate int
	ChunkSize                     int
	ParentPath                    string
	FileName                      string
	LastActivity                  time.Time
}

// need a function to notify a chunk has been uploaded
// need a function to reset

// still in intial state do not know what this function should return
// I will have to consider using locks
// because the same entry could be accessed by multiple go routine
func (u *UploadSessionState) NotifyConfirmation() {
	// keeping this function very simple right now
	u.ChunksUploaded++
	u.ChunksUploadedSinceLastUpdate++
}
func NewUploadSessionState(conn net.Conn, uploadID string, totalChunks int, chunkSize int, parentPath, fileName string) *UploadSessionState {
	return &UploadSessionState{
		Conn:         conn,
		UploadID:     uploadID,
		TotalChunks:  totalChunks,
		ChunkSize:    chunkSize,
		ParentPath:   parentPath,
		FileName:     fileName,
		LastActivity: time.Now(),
	}
}
