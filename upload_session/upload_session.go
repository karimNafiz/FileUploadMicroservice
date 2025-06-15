package upload_session

import (
	"net"
	"time"
)

type UploadSessionState struct {
	Conn                          net.Conn
	TotalChunks                   int
	ChunksUploaded                int
	ChunksUploadedSinceLastUpdate int
	ChunkSize                     int
	LastActivity                  time.Time
}
