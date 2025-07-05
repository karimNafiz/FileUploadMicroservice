package tcp_core

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

	//p_chunk_job "github.com/file_upload_microservice/chunk_job"
	"github.com/file_upload_microservice/global_configs"
	"github.com/file_upload_microservice/safemap"
	p_upload_request "github.com/file_upload_microservice/upload_request"
	p_upload_session "github.com/file_upload_microservice/upload_session"
)

// TODO: need to encapsulate the upload session in the upload session struct
// expose functions to start a session, by accepting a connection
// maybe convert the connection to bWriter, to buffer the bytes to be written
// expose functions like, Close(), Complete(),
// encapsulate the states using make shift "enums"

// BIG TODO: right now the code is running on the basis that everything will run smooth
// need to implement three sets in the upload session, that hold the chunk no's
// one set for errors, one set for acks, one set for the chunk_no

// need a function for listening to tcp connections
func StartTCPListener(ctx context.Context, port string, safemap *safemap.SafeMap[*p_upload_request.UploadRequest]) {
	// listen on the port provided
	listener, err := net.Listen("tcp", port)
	// if err, panic to shut down this service
	if err != nil {
		panic(err)
	}
	// make sure to close the listener no matter what happens
	defer listener.Close()

	log.Printf("uploading service running on port %s \n ", port)
	// infinite foor loop
	// in an infinite loop keep on listening for new connection s
	for {
		// Accept a new connection
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to accept connection:", err)
			continue
		}

		// Handle the connection in a separate goroutine

		// instead of doing go func()
		// okay no we need the go handle_connection
		// in the handle_connection
		// we check if there are any problems with the connection or not
		// if its okay we create a upload_session

		go handle_connection_new(ctx, conn, safemap)
	}

}

// for the handle_connection, we need two safe_maps one for upload_session and one for upload_request
// in handle_request the client first sends a RequestUploadSessionStart
// the server responds with ok, or some other shit that can be figured out
// if we send ok, before that we need to create the upload session and also start the upload session
// the upload session should handle all of the uploading and shit

func handle_connection_new(ctx context.Context, conn net.Conn, safemap *safemap.SafeMap[*p_upload_request.UploadRequest]) {
	// prepare our JSON envelope
	response := struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}{}

	// helper to send the envelope and close out
	sendResponse := func() {
		if data, err := json.Marshal(response); err == nil {
			conn.Write(data)
		} else {
			// fallback in case JSON marshaling ever fails
			fmt.Fprintf(conn, `{"status":500,"message":"internal JSON error"}`)
		}
	}

	bReader := bufio.NewReader(conn)
	headerBuf, err := read_header(bReader, global_configs.HEADERlENGTH)
	if err != nil {
		response.Status = 500
		response.Message = "error reading header of the new upload session"
		sendResponse()
		return
	}

	var headerBody struct {
		UploadID      string `json:"upload_id"`
		OperationCode uint8  `json:"operation_code"`
		ChunkNo       int    `json:"chunk_no"`
		ChunkSize     int    `json:"chunk_size"`
	}
	if err := json.Unmarshal(headerBuf, &headerBody); err != nil {
		response.Status = 400
		response.Message = "invalid header JSON payload"
		sendResponse()
		return
	}

	uploadReq, ok := safemap.Get(headerBody.UploadID)
	if !ok {
		response.Status = 404
		response.Message = "upload session not registered"
		sendResponse()
		return
	}

	// everything’s good — send an “OK” and proceed
	// if everything is found

	// create the UploadSession
	// right now I'm hard coding the 16, but create a const in the global_configs
	p_upload_session.CreateUploadSession(conn, bReader, uploadReq, global_configs.CHUNKJOBCONFIRMATIONWORKERPOOL*2, 16, 16).Start(ctx)

	// start the UploadSession
	// TODO learn the front-end shit in type-script
	// handle messaging between front and backend
	// I was starting the upload session before sending the response maybe start it after
	// this will solve the misalignement
	// response.Status = 200
	// response.Message = "upload session found"
	// sendResponse()

	// …now carry on with uploadReq, headerBody.OperationCode, etc…
}

// TODO:
// this is being used in multiple places
// move this to a utils package
// im too lazy to move it now xD
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

// func read_chunk(bReader *bufio.Reader, chunk_size int) ([]byte, error) {
// 	chunk_buffer := make([]byte, chunk_size)
// 	n, err := io.ReadFull(bReader, chunk_buffer[:])

// 	if n < chunk_size {
// 		return nil, errors.New(" not all of the chunk is sent ")
// 	}
// 	if err != nil {

// 		return nil, errors.New("reading chunk error")
// 	}

// 	return chunk_buffer, nil

// }

// func init_upload_session(ctx context.Context, upload_session *upload_session.UploadSession, tcp_socket net.Conn, session_pool_size uint, session_error_pool_size uint, session_ack_pool_size uint) error {
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

// func handle_connection(conn net.Conn, safemap *safemap.SafeMap[*upload_session.UploadSession]) {
// 	// create a buffered reader out of the connection
// 	// make sure to close the connection after the session is complete
// 	// TODO, need timeouts for the session, if the session has no activity in certain period of time close the connection

// 	// need a context variable so that all the go-routines that have spawned from the co-routine holding the context
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer conn.Close()
// 	// create a buffered reader my fav thing
// 	// without a buffered reader I don't know what are you gonna do
// 	bReader := bufio.NewReader(conn)

// 	// read fixed size header
// 	// operation code 0 first chunk, 1 upload chunk, 2 close connection upload complete, 3 close connection upload not complete
// 	// from the header get the uploadID
// 	// chunkNO,
// 	var header_body struct {
// 		UploadID      string `json:"upload_id"`
// 		OperationCode uint8  `json:"operation_code"`
// 		ChunkNo       int    `json:"chunk_no"`
// 		ChunkSize     int    `json:"chunk_size"`
// 	}
// 	for {
// 		header_buffer, err := read_header(bReader, global_configs.HEADERlENGTH)
// 		if err != nil {
// 			fmt.Println("error reading header bytes err:1")
// 			fmt.Println(err.Error())
// 		}

// 		// decode the buffer into an map object
// 		err = json.Unmarshal(header_buffer, &header_body)

// 		if err != nil {
// 			fmt.Println("error unmarshalling the header err:2 ")
// 			fmt.Println(err.Error())
// 		}
// 		// i don't really like this code
// 		// TODO find a better solution than getting the upload_session from the safe map everytime we get message
// 		upload_session_state, flag := safemap.Get(header_body.UploadID)
// 		if !flag {
// 			fmt.Println("error could not find the upload session err:3")
// 		}
// 		//log.Println("got the upload_session_related to the connection ")
// 		//log.Println("upload session upload ID " + header_body.UploadID)
// 		switch header_body.OperationCode {
// 		case global_configs.UPLOADINITOPCODE:

// 			if err := init_upload_session(ctx, upload_session_state, conn, global_configs.CHUNKJOBWORKERPOOL*2, 16, 16); err != nil {
// 				// need to do something
// 				// this error means the Upload Session Doesn't exist
// 			}
// 			//fmt.Println("!!!!! Initialized the upload_session !!!!!!!!!!!!!!")
// 		case global_configs.UPLOADCHUNKOPCODE:
// 			chunk_buffer, err := read_chunk(bReader, header_body.ChunkSize)
// 			if err != nil {
// 				// issues with reading chunks
// 				// need to send the sender a message
// 				log.Println("error reading chunk:", err)
// 				upload_session_state.Err <- &p_chunk_job.ChunkJobError{UploadID: header_body.UploadID, ChunkNo: uint(header_body.ChunkNo), Error: err}
// 				continue
// 			}
// 			chunk_job := p_chunk_job.CreateChunkJob(header_body.UploadID, uint(header_body.ChunkNo), upload_session_state.ParentPath, chunk_buffer, upload_session_state.Acks, upload_session_state.Err)
// 			// need to add this chunk job to the thread pool
// 			upload_session_state.In <- chunk_job
// 		// do not think i need this
// 		// keep sets in the upload session
// 		// case global_configs.UPLOADRETRANSMISSIONOPCODE:
// 		// 	// need to handle case where there was re-transmission
// 		case global_configs.UPLOADFINISHOPCODE:
// 			// if the uploading is complete it doesn't matter
// 			// because the front-end can do re-transmission
// 			// check if all the chunks were uploaded or not
// 			if !upload_session_state.IsComplete {
// 				// not all chunks confirmed yet: ask client to wait or retry missing
// 				// this is temporary
// 				msg := map[string]string{"error": "upload not complete"}
// 				b, _ := json.Marshal(msg)
// 				conn.Write(b)
// 				continue
// 			}
// 			// signal session shutdown
// 			upload_session_state.Done <- struct{}{}
// 			// send final complete notice
// 			// maybe send status codes
// 			complete := map[string]string{
// 				"upload_id": header_body.UploadID,
// 				"status":    "complete",
// 			}
// 			b, _ := json.Marshal(complete)
// 			conn.Write(b)
// 			// need to cancel the context to signal other go-routines to also stop
// 			cancel()
// 			return

// 		case global_configs.UPLOADCANCELOPCODE:
// 			// client requests abort: tear down session
// 			upload_session_state.Done <- struct{}{}
// 			// send cancelled notification
// 			cancelMsg := map[string]string{"upload_id": header_body.UploadID, "status": "cancelled"}
// 			b, _ := json.Marshal(cancelMsg)
// 			conn.Write(b)
// 			// need to cancel the context to signal other go-routines to also stop
// 			cancel()
// 			return
// 		default:
// 			log.Println("unknown operation code:", header_body.OperationCode)

// 		}

// 	}

// }
