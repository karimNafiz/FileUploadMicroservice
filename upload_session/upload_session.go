package upload_session

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"time"

	p_chunk_job "github.com/file_upload_microservice/chunk_job"
	p_upload_request "github.com/file_upload_microservice/upload_request"
)

// for this upload session I need to have
// channel chunk request
// channel chunk error
// channel chunk acks
// channel done (not sure about this one)
type UploadSession struct {
	// UploadID                      string
	// TotalChunks                   int
	// ChunkSize                     int
	// ParentPath                    string
	// FileName                      string
	UploadRequest                 *p_upload_request.UploadRequest
	LastActivity                  time.Time
	ChunksUploaded                int
	ChunksUploadedSinceLastUpdate int
	// TODO this IsComplete is a temporary fix
	// this is not sustainable, but current I have to use quick solutions to finish this project
	IsComplete bool

	// important for session state
	// i can think of better names later
	Writer  net.Conn
	Reader  *bufio.Reader
	In      chan *p_chunk_job.ChunkJob
	Err     chan *p_chunk_job.ChunkJobError
	Acks    chan *p_chunk_job.ChunkJobAck
	Context context.Context
	Done    chan struct{}
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

	u.check_if_upload_complete()
}
func (u *UploadSession) check_if_upload_complete() {
	// if chunks uploaded less than
	// total chunks
	// that means uploading not complete
	if u.ChunksUploaded < u.UploadRequest.TotalChunks {
		return
	}
	// if uploading complete
	// then we push a empty struct{} onto the channel

	// this code will block for sure
	// REMEMBER THIS, this is why AI is not the answer
	u.IsComplete = true
	u.Done <- struct{}{}

}

func CreateUploadSession(ctx context.Context, conn net.Conn, reader *bufio.Reader, upload_request *p_upload_request.UploadRequest, session_pool_size int, ack_pool_size int, err_pool_size int) *UploadSession {
	return &UploadSession{
		Context:       ctx,
		UploadRequest: upload_request,
		Writer:        conn,
		Reader:        reader,
		IsComplete:    false, // TODO change this to some type of enum to keep track of the state
		In:            make(chan *p_chunk_job.ChunkJob, session_pool_size),
		Acks:          make(chan *p_chunk_job.ChunkJobAck, ack_pool_size),
		Err:           make(chan *p_chunk_job.ChunkJobError, err_pool_size),
	}
}

func (upload_session *UploadSession) Start() {
	// in the start loop for fuck sake

	go func() {
		// typical go for select loop
		// need to add a timeout
		// TODO: review the entire system
		// to find out where exactly I need to add timeouts

		// create an encoder
		// the encoder will be re-used for our purpose
		for {
			select {
			case chunk_job := <-upload_session.In:
				fmt.Println("added chunk job from upload_session.In into the chunk job ")
				fmt.Println(" chunk job: " + chunk_job.String())
				p_chunk_job.AddChunkJob(chunk_job)
			// maybe instead of hard coding this error I need to find a better solution
			// maybe have encode functions for those structs?
			case chunk_job_error := <-upload_session.Err:
				bytes, err := chunk_job_error.MarshalJSON()
				if err != nil {
					// don't really what to do in this case
				}
				upload_session.Writer.Write(bytes)
				// write back to the connection
			case chunk_job_ack := <-upload_session.Acks:
				// when there is an ack
				// i need to notify the upload session
				// add a buffering
				bytes, err := chunk_job_ack.MarshalJSON()
				if err != nil {
					// don't know what to really do
				}
				upload_session.NotifyConfirmation()
				// do no simply write every chunk at once maybe
				upload_session.Writer.Write(bytes)
			case <-upload_session.Done:
				return
			case <-upload_session.Context.Done():
				return
			}

		}
	}()

}

// 	// TODO consider creating a buffered Writer from the conn in here
// 	// first read about bufferedWriter toh

// 	upload_session.Conn = tcp_socket
// 	upload_session.In = make(chan *p_chunk_job.ChunkJob, session_pool_size)
// 	upload_session.Err = make(chan *p_chunk_job.ChunkJobError, session_error_pool_size)
// 	upload_session.Acks = make(chan *p_chunk_job.ChunkJobAck, session_ack_pool_size)
// 	// right now hard coding this one
// 	// without this one
// 	// this will not be a buffered channel
// 	// and the session is managed by a single go-routine
// 	// so if the go-routine pushesh to the done channel
// 	// it will be parked
// 	// TODO learn about the parking mechanism again
// 	upload_session.Done = make(chan struct{}, 1)

// 	// this dispatcher go function
// 	go func() {
// 		// typical go for select loop
// 		// need to add a timeout
// 		// TODO: review the entire system
// 		// to find out where exactly I need to add timeouts

// 		// create an encoder
// 		// the encoder will be re-used for our purpose
// 		for {
// 			select {
// 			case chunk_job := <-upload_session.In:
// 				fmt.Println("added chunk job from upload_session.In into the chunk job ")
// 				fmt.Println(" chunk job: " + chunk_job.String())
// 				p_chunk_job.AddChunkJob(chunk_job)
// 			// maybe instead of hard coding this error I need to find a better solution
// 			// maybe have encode functions for those structs?
// 			case chunk_job_error := <-upload_session.Err:
// 				bytes, err := chunk_job_error.MarshalJSON()
// 				if err != nil {
// 					// don't really what to do in this case
// 				}
// 				tcp_socket.Write(bytes)
// 				// write back to the connection
// 			case chunk_job_ack := <-upload_session.Acks:
// 				// when there is an ack
// 				// i need to notify the upload session
// 				// add a buffering
// 				bytes, err := chunk_job_ack.MarshalJSON()
// 				if err != nil {
// 					// don't know what to really do
// 				}
// 				upload_session.NotifyConfirmation()
// 				// do no simply write every chunk at once maybe
// 				tcp_socket.Write(bytes)
// 			case <-upload_session.Done:
// 				return
// 			case <-ctx.Done():
// 				return
// 			}

// 		}
// 	}()

// 	return nil
// }

// func NewUploadSession(conn net.Conn, uploadID string, totalChunks int, chunkSize int, parentPath, fileName string) *UploadSession {
// 	return &UploadSession{
// 		Conn:         conn,
// 		UploadID:     uploadID,
// 		TotalChunks:  totalChunks,
// 		ChunkSize:    chunkSize,
// 		ParentPath:   parentPath,
// 		FileName:     fileName,
// 		LastActivity: time.Now(),
// 		IsComplete:   false,
// 	}
// }

// need a function for Start()
// need a function NewUploadSession()
//
