package main

import (
	"context"
	"encoding/json"
	"log"

	"net/http"

	p_chunk_job "github.com/file_upload_microservice/chunk_job"
	p_safemap "github.com/file_upload_microservice/safemap"
	p_tcp_core "github.com/file_upload_microservice/tcp_core"
	p_upload_session_state "github.com/file_upload_microservice/upload_session"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	// need to set up the main router

	// instantiating the buffered chunk job channel
	// currently hard coding it to four
	// should change it later
	p_chunk_job.InstantiateBufferedChunkJobChannel(10)

	// now need to start the worker pool
	// for that need to create a background context so that I can stop the context and all the derived go routines
	ctx, cancel := context.WithCancel(context.Background())

	// now need to start creating the worker pool
	err := p_chunk_job.StartWorkerPool(ctx, 4)

	// if there is an error cancel the context
	if err != nil {
		cancel()
		return
	}
	// now need to start the error checking worker pool
	// need to change this code
	// pass a callback
	err = p_chunk_job.StartErrorHandlerPool(ctx, 4)
	if err != nil {
		cancel()
		return
	}

	// need to start the chunk_job_confirmation worker pool

	// create the in memory safe map
	// the safe map will be of type pointer to UploadSesionState
	// the UploadSession holds all the necessary information about the upload session
	safemap := p_safemap.NewSafeMap[*p_upload_session_state.UploadSession]()

	// need to set up the main router
	router := setUpRouter(safemap)

	// need to launch this service in a different go-routine or else
	// no code will run below this code
	go func() {
		// now need to start the tcp connection
		// currently start the tcp listener on port 9000
		// currently hard coding it, need to change it later
		// passing the safe map created
		p_tcp_core.StartTCPListener(":9000", safemap)

	}()

	cors := handlers.CORS(
		handlers.AllowedOrigins([]string{"http://localhost:3000"}), // your UI origin
		handlers.AllowedMethods([]string{"GET", "POST", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "X-Chunk-Index"}),
		handlers.AllowCredentials(),
	)

	log.Println(" Main Server running on :8000 ")
	http.ListenAndServe(":8000", cors(router))

}

func setUpRouter(safemap *p_safemap.SafeMap[*p_upload_session_state.UploadSession]) *mux.Router {
	router := mux.NewRouter()
	router.Handle("/upload/init", getInitUploadSessionHandler(safemap)).Methods("POST")
	return router
}

func getInitUploadSessionHandler(safemap *p_safemap.SafeMap[*p_upload_session_state.UploadSession]) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// need to make sure the connection is closed
		defer r.Body.Close()
		// struct to represent the request
		// to send a request a the API, they need to follow this structure
		var reqBody struct {
			UploadID    string `json:"uploadID"`
			Filename    string `json:"filename"`
			FinalPath   string `json:"final_path"`
			ChunkSize   int    `json:"chunk_size"`
			TotalChunks int    `json:"total_chunks"`
		}
		// decoding the body
		err := json.NewDecoder(r.Body).Decode(&reqBody)

		// if there were errors whilst decoding, then the request is bad
		if err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		// if there are not errors in the request body need to create a NewUploadSession struct
		// I am manually adding the UploadSession
		// create a NewUploadSession
		// TODO need to add some safety measures
		safemap.Add(reqBody.UploadID, p_upload_session_state.NewUploadSession(nil, reqBody.UploadID, reqBody.TotalChunks, reqBody.ChunkSize, reqBody.FinalPath, reqBody.Filename))

		// after adding it to the safe map need to send a message back to the client

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Upload Session Created",
		})

	})
}

// func initUploadSession(w http.ResponseWriter, r *http.Request){
// 	// need to make sure the connection is closed
// 	defer r.Body.Close()
// 	var reqBody struct{
// 		UploadID string `json:"uploadID"`
// 		Filename string `json:"filename"`
// 		FinalPath string `json:"final_path"`
// 		ChunkSize int `json:"chunk_size"`
// 		TotalChunks int `json:"total_chunks"`
// 	}

// 	err := json.NewDecoder(r.Body).Decode(&reqBody)

// 	if err != nil{
// 		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
// 		return
// 	}

// }
