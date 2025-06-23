package main

import (
	"context"

	p_chunk_job "github.com/file_upload_microservice/chunk_job"
	p_safemap "github.com/file_upload_microservice/safemap"
	p_tcp_core "github.com/file_upload_microservice/tcp_core"
	p_upload_session_state "github.com/file_upload_microservice/upload_session"
)

func main() {

	// instantiating the buffered chunk job channel
	// currently hard coding it to four
	// should change it later
	p_chunk_job.InstantiateBufferedChunkJobChannel(10)

	// now need to start the worker pool
	// for that need to create a background context so that I can stop the context and all the derived go routines
	ctx, _ := context.WithCancel(context.Background())

	// now need to start creating the worker pool
	p_chunk_job.StartWorkerPool(ctx, 4)
	// now need to start the error checking worker pool
	p_chunk_job.StartErrorHandlerPool(ctx, 4)

	// need to start the chunk_job_confirmation worker pool

	// create the in memory safe map
	// the safe map will be of type pointer to UploadSesionState
	// the UploadSessionState holds all the necessary information about the upload session
	safemap := p_safemap.NewSafeMap[*p_upload_session_state.UploadSessionState]()

	// now need to start the tcp connection
	// currently start the tcp listener on port 9000
	// currently hard coding it, need to change it later
	// passing the safe map created
	p_tcp_core.StartTCPListener(":9000", safemap)

}
