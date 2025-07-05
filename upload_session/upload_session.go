package upload_session

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	p_chunk_job "github.com/file_upload_microservice/chunk_job"
	"github.com/file_upload_microservice/global_configs"
	p_upload_request "github.com/file_upload_microservice/upload_request"
)

/*
	HOW THE ENTIRE SYSTEM WORKS
	AN UPLOAD SESSION IS IN CHARGE OF TAKING CARE OF AN UPLOAD SESSION
	IT READS THE DATA IN CHUNKS AND APPROPRIATELY PUTS IN A ONE OF ITS CHANNELS
	IN MEAN READY TO DOWNLOADED
	ANOTHER GO-ROUTINE IS IN CHARGE OF TAKING THE CHUNKS FROM IN TO THE CHUNK UPLOAD WORKER POOL
	THE REASON FOR THIS DESIGN, IS SO THAT THE UPLOAD SESSION CAN SOLELY FOCUS ON READING DATA FROM THE CONNECTION
	DOESN'T HAVE TO CONCERN ITSELF IN ACTUALLY GIVING THE CHUNKS TO THE WORKER POOL BECAUSE,
	IMAGINE THERE ARE X WORKERS, AND IF X OF THEM ARE BUSY THEN THE UPLOAD SESSION WOULD BLOCK
	SO TO NEGATE THAT SITUATION, THE UPLOAD SESSION HAS AN "IN" CHANNEL WHICH IS LARGER THAN THE WORKER POOL
	SO THAT IT PROVIDES A BUFFER
*/

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
func (u *UploadSession) update_session() {
	// keeping this function very simple right now
	u.ChunksUploaded++
	u.ChunksUploadedSinceLastUpdate++

	u.check_if_upload_complete()
}
func (u *UploadSession) check_if_upload_complete() {
	// if chunks uploaded less than
	// total chunks
	// that means uploading not complete
	fmt.Printf("total chunks uploaded %d ", u.ChunksUploaded)

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

func CreateUploadSession(conn net.Conn, reader *bufio.Reader, upload_request *p_upload_request.UploadRequest, session_pool_size int, ack_pool_size int, err_pool_size int) *UploadSession {
	return &UploadSession{
		UploadRequest: upload_request,
		Writer:        conn,
		Reader:        reader,
		IsComplete:    false, // TODO change this to some type of enum to keep track of the state
		In:            make(chan *p_chunk_job.ChunkJob, session_pool_size),
		Acks:          make(chan *p_chunk_job.ChunkJobAck, ack_pool_size),
		Err:           make(chan *p_chunk_job.ChunkJobError, err_pool_size),
		Done:          make(chan struct{}, 1), // very important need to make it of size 1 or-else you are fucked or else there will be a deadlock
	}
}

// to the start function pass the context
func (upload_session *UploadSession) Start(ctx context.Context) {
	// in the start loop for fuck sake

	go upload_session.handle_upload_session_channels(ctx)
	go upload_session.read_frm_conn(ctx)

}

func (upload_session *UploadSession) handle_upload_session_channels(ctx context.Context) {
	for {
		select {
		case chunk_job := <-upload_session.In:
			//fmt.Println("added chunk job from upload_session.In into the chunk job ")
			//fmt.Println(" chunk job: " + chunk_job.String())
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
			upload_session.update_session()
			// do no simply write every chunk at once maybe
			upload_session.Writer.Write(bytes)
		case <-upload_session.Done:
			fmt.Println("stdout from upload_session.Done channel ")
			fmt.Println("all chunks written onto disk")
			return
		case <-ctx.Done():
			// when the context is done need to somehow contact the front-end client
			// TODO: push to the Done channel of the upload_session
			return

		}

	}

}

// need to added context
// to stop the entire shit
// if the main service falls
func (u *UploadSession) read_frm_conn(ctx context.Context) {

	var header_body struct {
		UploadID      string `json:"upload_id"`
		OperationCode uint8  `json:"operation_code"`
		ChunkNo       int    `json:"chunk_no"`
		ChunkSize     int    `json:"chunk_size"`
	}
	for {
		// I need to handle the situation where no data is sent
		// ill do this later after I'm done cleaning up

		select {
		case <-ctx.Done():
			fmt.Println("context cancelled in read_frm_conn:", ctx.Err())
			// need to return a error back to the client
			return
		default:
			header_buffer, err := read_header(u.Reader, global_configs.HEADERlENGTH)
			if err != nil {
				fmt.Println("error reading header bytes ")
				fmt.Println(err.Error())
			}

			// decode the buffer into an map object
			err = json.Unmarshal(header_buffer, &header_body)

			if err != nil {
				fmt.Println("error unmarshalling the header ")
				fmt.Println(err.Error())
			}

			switch header_body.OperationCode {

			case global_configs.UPLOADCHUNKOPCODE:
				// read the chunk
				chunk_buffer, err := read_chunk(u.Reader, header_body.ChunkSize)
				// if there is an error
				// send it to the error channel
				if err != nil {
					// issues with reading chunks
					// need to send the sender a message
					log.Println("error reading chunk:", err)
					u.Err <- &p_chunk_job.ChunkJobError{UploadID: header_body.UploadID, ChunkNo: uint(header_body.ChunkNo), Error: err}
					continue
				}
				// create a chunk job
				chunk_job := p_chunk_job.CreateChunkJob(header_body.UploadID, uint(header_body.ChunkNo), u.UploadRequest.ParentPath, chunk_buffer, u.Acks, u.Err)
				// add it to the upload_session's in channel
				u.In <- chunk_job
			case global_configs.UPLOADFINISHOPCODE:
				// after the client has recieved acks for all the chunks
				// its going to want to finish upload
				if !u.IsComplete {
					// not all chunks confirmed yet: ask client to wait or retry missing
					// this is temporary
					// TODO
					// bring this to the proper format
					msg := map[string]string{"error": "upload not complete"}
					b, _ := json.Marshal(msg)
					u.Writer.Write(b)
					continue
				}
				// if the upload_session is also complete
				u.Done <- struct{}{}
				// send final complete notice
				// maybe send status codes
				// TOOD get appropriate status code
				complete := map[string]string{
					"upload_id": header_body.UploadID,
					"status":    "complete",
				}
				b, _ := json.Marshal(complete)
				u.Writer.Write(b)
				// need to cancel the context to signal other go-routines to also stop
				// cancel the context
				return

			case global_configs.UPLOADCANCELOPCODE:
				// client requests abort: tear down session
				u.Done <- struct{}{}
				// send cancelled notification
				// TODO : get appropriate status code
				cancelMsg := map[string]string{"upload_id": header_body.UploadID, "status": "cancelled"}
				b, _ := json.Marshal(cancelMsg)
				u.Writer.Write(b)
				// need to cancel the context to signal other go-routines to also stop
				return
			default:
				log.Println("unknown operation code:", header_body.OperationCode)

			}
		}

	}

}

func read_header(bReader *bufio.Reader, header_len int) ([]byte, error) {
	// make a buffer for reading the header bytes
	header_len_buffer := make([]byte, header_len)
	// actually the header bytes
	n, err := io.ReadFull(bReader, header_len_buffer[:])
	// checking for errors
	if n < header_len {
		// return error not all bytes were read
		return nil, errors.New("not all header len bytes were read")
	}
	// checking for errors
	if err != nil {
		// some io error
		return nil, err
	}
	// at this point we know the header length
	header_len = int(binary.BigEndian.Uint32(header_len_buffer))

	header_buffer := make([]byte, header_len)

	n, err = io.ReadFull(bReader, header_buffer)

	if n < header_len {
		// return error that not all bytes were returned
		return nil, errors.New("not all header bytes were read")
	}
	if err != nil {
		return nil, err
	}

	return header_buffer, nil

}

func read_chunk(bReader *bufio.Reader, chunk_size int) ([]byte, error) {
	chunk_buffer := make([]byte, chunk_size)
	n, err := io.ReadFull(bReader, chunk_buffer[:])

	if n < chunk_size {
		return nil, errors.New(" not all of the chunk is sent ")
	}
	if err != nil {

		return nil, errors.New("reading chunk error")
	}

	return chunk_buffer, nil

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
// 				upload_session.update_session()
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
