package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"net/http"

	p_chunk_job "github.com/file_upload_microservice/chunk_job"
	"github.com/file_upload_microservice/global_configs"
	p_global_configs "github.com/file_upload_microservice/global_configs"
	p_safemap "github.com/file_upload_microservice/safemap"
	p_tcp_core "github.com/file_upload_microservice/tcp_core"
	p_upload_request "github.com/file_upload_microservice/upload_request"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	//TODO when the code is refactored this package shouldn't be here
	p_registered_service "github.com/file_upload_microservice/registered_service"
)

// TODO: refactor this file
// rn I have put the surface level work in this entire file
// but if it starts getting too big ill create different packages for the router and handlers

func main() {
	// need to set up the main router

	// instantiating the buffered chunk job channel
	// currently hard coding it to four
	// should change it later
	p_chunk_job.InstantiateBufferedChunkJobChannel(p_global_configs.CHUNKJOBCHANNELBUFFERSIZE)

	// now need to start the worker pool
	// for that need to create a background context so that I can stop the context and all the derived go routines
	ctx, cancel := context.WithCancel(context.Background())

	// now need to start creating the worker pool
	err := p_chunk_job.StartWorkerPool(ctx, global_configs.CHUNKJOBWORKERPOOL)

	// if there is an error cancel the context
	if err != nil {
		cancel()
		return
	}
	// now need to start the error checking worker pool
	// need to change this code
	// pass a callback
	err = p_chunk_job.StartErrorHandlerPool(ctx, global_configs.CHUNKJOBERRPOOL)
	if err != nil {
		cancel()
		return
	}

	err = p_chunk_job.StartJobConfirmationHandlerPool(ctx, global_configs.CHUNKJOBCONFIRMATIONWORKERPOOL)
	if err != nil {
		cancel() // this is the context
		return
	}

	// need to start the chunk_job_confirmation worker pool

	// create the in memory safe map
	// the safe map will be of type pointer to UploadSesionState
	// the UploadSession holds all the necessary information about the upload session
	safemap := p_safemap.NewSafeMap[*p_upload_request.UploadRequest]()

	// need a map of services registered to file_upload_service
	// this map will store all the services registered to the file_upload_service
	// TODO maybe in the future to implement rate limiting
	// we can have a monitor go-routine
	service_map := p_safemap.NewSafeMap[*p_registered_service.Service]()

	// need to set up the main router
	router := setUpRouter(safemap, service_map)

	// need to launch this service in a different go-routine or else
	// no code will run below this code
	go func() {
		// now need to start the tcp connection
		// currently start the tcp listener on port 9000
		// currently hard coding it, need to change it later
		// passing the safe map created
		p_tcp_core.StartTCPListener(ctx, ":9000", safemap)

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

func setUpRouter(safemap *p_safemap.SafeMap[*p_upload_request.UploadRequest], service_map *p_safemap.SafeMap[*p_registered_service.Service]) *mux.Router {
	router := mux.NewRouter()
	router.Handle("/upload/init", getInitUploadSessionHandler(safemap)).Methods("POST")
	router.Handle("/register", GetRegisterToFileUploadService(service_map))
	return router
}

// take in the safemap
func GetRegisterToFileUploadService(service_map *p_safemap.SafeMap[*p_registered_service.Service]) http.Handler {
	get_service_id := start_service_id(-1)
	// need a handler for main services to register to the file-upload service
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// very important to avoid memory leaks
		defer r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		var reqBody struct {
			Host                    string `json:"host"`
			Scheme                  string `json:"scheme"`
			Port                    string `json:"port"`
			UploadStatusCallBackURL string `json:"upload_status_callback_url"`
		}

		err := json.NewDecoder(r.Body).Decode(&reqBody)
		// any error with decoding the header body
		if err != nil {
			// once the header is sent we can't change the header
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "bad request body",
			})
		}
		// if not error decoding the body
		// we need to create a new service
		service_id := get_service_id()
		service := p_registered_service.NewService(service_id, reqBody.Host, reqBody.Scheme, reqBody.Port, reqBody.UploadStatusCallBackURL)
		// after creating the service add it to the safemap
		// TODO implement the ID check if the id already exists
		// for our simple case that won't be the issue
		// but when we implement the goodleuuid, then do check
		// even though the chances are astronomically low
		service_map.Add(service_id, service)

		// after adding the service we need to let the main service know habibi you have been added
		// maybe we change to smth else
		// TODO add the security feature
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"service_id": service_id,
			"message":    "service registered",
		})

	})
}

func start_service_id(start_index int) func() string {
	return func() string {
		start_index++
		return fmt.Sprintf("service:%d", start_index)
	}
}

func getInitUploadSessionHandler(safemap *p_safemap.SafeMap[*p_upload_request.UploadRequest]) http.Handler {

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
		safemap.Add(reqBody.UploadID, &p_upload_request.UploadRequest{
			UploadID:    reqBody.UploadID,
			FileName:    reqBody.Filename,
			ParentPath:  reqBody.FinalPath,
			TotalChunks: reqBody.TotalChunks,
			ChunkSize:   reqBody.ChunkSize,
		})

		// after adding it to the safe map need to send a message back to the client

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Upload Request Created",
		})

	})
}

// need a post request for the main service to register itself with the file upload service
// create a channel per-service
// example scenario, main service registers with the file upload service
// the file upload_service creates a channel for that 'main_service'
//

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
