package tcp_core

import (
	"bufio"
	"fmt"
	"io"
	"net"
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

	for {
		// read fixed size header
		// operation code 0 first chunk, 1 upload chunk, 2 close connection upload complete, 3 close connection upload not complete
		// from the header get the uploadID
		// chunkNO,

	}

}

func read_header(bReader *bufio.Reader, header_len int) ([]byte, error) {
	header_len_buffer := make([]byte, header_len)
	n, err := io.ReadFull(bReader, header_len_buffer[:])

}
