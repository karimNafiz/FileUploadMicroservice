package tcp_core

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"

	p_chunk_job "github.com/file_upload_microservice/chunk_job"
	"github.com/file_upload_microservice/global_configs"
	"github.com/file_upload_microservice/safemap"
	"github.com/file_upload_microservice/upload_session"
)

// need a function for listening to tcp connections
func StartTCPListener(port string, safemap *safemap.SafeMap[*upload_session.UploadSession]) {
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
		go handle_connection(conn, safemap)
	}

}

func handle_connection(conn net.Conn, safemap *safemap.SafeMap[*upload_session.UploadSession]) {
	// create a buffered reader out of the connection
	// make sure to close the connection after the session is complete
	// TODO, need timeouts for the session, if the session has no activity in certain period of time close the connection

	defer conn.Close()
	// create a buffered reader my fav thing
	// without a buffered reader I don't know what are you gonna do
	bReader := bufio.NewReader(conn)

	// read fixed size header
	// operation code 0 first chunk, 1 upload chunk, 2 close connection upload complete, 3 close connection upload not complete
	// from the header get the uploadID
	// chunkNO,
	var header_body struct {
		UploadID      string `json:"upload_id"`
		OperationCode uint8  `json:"operation_code"`
		ChunkNo       int    `json:"chunk_no"`
		ChunkSize     int    `json:"chunk_size"`
	}
	for {
		header_buffer, err := read_header(bReader, global_configs.HEADERlENGTH)
		if err != nil {
			fmt.Println("error reading header bytes err:1")
			fmt.Println(err.Error())
		}

		// decode the buffer into an map object
		err = json.Unmarshal(header_buffer, &header_body)

		if err != nil {
			fmt.Println("error unmarshalling the header err:2 ")
			fmt.Println(err.Error())
		}
		fmt.Println("unmarshalled header ")
		fmt.Println(header_body)

		// i don't really like this code
		// TODO find a better solution than getting the upload_session from the safe map everytime we get message
		upload_session_state, flag := safemap.Get(header_body.UploadID)
		if !flag {
			fmt.Println("error could not find the upload session err:3")
		}
		log.Println("got the upload_session_related to the connection ")
		log.Println("upload session upload ID " + header_body.UploadID)
		switch header_body.OperationCode {
		case global_configs.UPLOADINITOPCODE:
			if err := init_upload_session(header_body.UploadID, safemap, conn, global_configs.CHUNKJOBWORKERPOOL*2, 16, 16); err != nil {
				// need to do something
				// this error means the Upload Session Doesn't exist
			}
		case global_configs.UPLOADCHUNKOPCODE:

			chunk_buffer, err := read_chunk(bReader, header_body.ChunkSize)
			if err != nil {
				// issues with reading chunks
				// need to send the sender a message
			}
			chunk_job := p_chunk_job.CreateChunkJob(header_body.UploadID, uint(header_body.ChunkNo), upload_session_state.ParentPath, chunk_buffer, upload_session_state.Acks, upload_session_state.Err)
			// need to add this chunk job to the thread pool
			p_chunk_job.AddChunkJob(chunk_job)
		case global_configs.UPLOADFINISHOPCODE:
			// check if all the chunks were uploaded or not
		case global_configs.UPLOADCANCELOPCODE:
			// need to do clean up
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

func init_upload_session(uploadID string, safemap *safemap.SafeMap[*upload_session.UploadSession], tcp_socket net.Conn, session_pool_size uint, session_error_pool_size uint, session_ack_pool_size uint) error {
	// TODO consider creating a buffered Writer from the conn in here
	// first read about bufferedWriter toh

	// need to remove this code and send the upload session as an function argument
	// becuase I am already finding out the upload session
	// TODO : read the lines above
	upload_session, exists := safemap.Get(uploadID)
	if !exists {
		return errors.New("UploadSession does not exist")
	}
	// if no error
	// then we have to add all the session data to the UploadSession

	upload_session.Conn = tcp_socket
	upload_session.In = make(chan *p_chunk_job.ChunkJob, session_pool_size)
	upload_session.Err = make(chan *p_chunk_job.ChunkJobError, session_error_pool_size)
	upload_session.Acks = make(chan *p_chunk_job.ChunkJobAck, session_ack_pool_size)
	// right now hard coding this one
	// without this one
	// this will not be a buffered channel
	// and the session is managed by a single go-routine
	// so if the go-routine pushesh to the done channel
	// it will be parked
	// TODO learn about the parking mechanism again
	upload_session.Done = make(chan struct{}, 1)

	// this dispatcher go function
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
				p_chunk_job.AddChunkJob(chunk_job)
			// maybe instead of hard coding this error I need to find a better solution
			// maybe have encode functions for those structs?
			case chunk_job_error := <-upload_session.Err:
				bytes, err := chunk_job_error.MarshalJSON()
				if err != nil {
					// don't really what to do in this case
				}
				tcp_socket.Write(bytes)
				// write back to the connection
			case chunk_job_ack := <-upload_session.Acks:
				// when there is an ack
				// i need to notify the upload session
				bytes, err := chunk_job_ack.MarshalJSON()
				if err != nil {
					// don't know what to really do
				}
				upload_session.NotifyConfirmation()
				tcp_socket.Write(bytes)
			case <-upload_session.Done:
				return
			}

		}
	}()

	return nil
}
