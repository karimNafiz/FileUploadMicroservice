package tcp_core

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/file_upload_microservice/global_configs"
)

// need a function for listening to tcp connections
func StartTCPListener(port string) {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	// infinite foor loop
	for {
		// Accept a new connection
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to accept connection:", err)
			continue
		}

		// Handle the connection in a separate goroutine
		go handle_connection(conn)
	}

}

func handle_connection(conn net.Conn) {
	// create a buffered reader out of the connection
	defer conn.Close()
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
			// let the user
		}

		// decode the buffer into an map object
		err = json.Unmarshal(header_buffer, &header_body)

		if err != nil {
			// request error need to let the sender know
		}

		switch header_body.OperationCode {

		case global_configs.UPLOADCHUNKOPCODE:
			// we add it to the channel
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
