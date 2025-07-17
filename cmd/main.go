package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

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

// TODO: refactor the package.
// Currently, this package has functions that do not belong here

// / <summary>
// / function used to load tls certificate and key
// / </summary>
func load_tls_cert_and_key(cert_dst string, key_dst string) (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(cert_dst, key_dst)
	if err != nil {
		log.Fatalf("could not load key pair: %v", err)
	}
	return cert, err
}

func main() {

	p_chunk_job.InstantiateBufferedChunkJobChannel(p_global_configs.CHUNKJOBCHANNELBUFFERSIZE)
	// need parent context
	// to stop cascading operations when the main go-routine stops
	ctx, cancel := context.WithCancel(context.Background())

	err := p_chunk_job.StartWorkerPool(ctx, global_configs.CHUNKJOBWORKERPOOL)

	// if there are errors with starting the worker pool
	// we return and cancel the context
	if err != nil {
		cancel()
		return
	}

	err = p_chunk_job.StartErrorHandlerPool(ctx, global_configs.CHUNKJOBERRPOOL)
	if err != nil {
		cancel()
		return
	}

	err = p_chunk_job.StartJobConfirmationHandlerPool(ctx, global_configs.CHUNKJOBCONFIRMATIONWORKERPOOL)
	if err != nil {
		cancel()
		return
	}

	// when a foreign service, requests an upload session
	// before starting an upload session we will create an upload request object
	// TODO: create a monitoring go-routine on this safe map
	// such that when an upload request stays in memory for too long we will remove it and let the foreign service know
	safemap := p_safemap.NewSafeMap[*p_upload_request.UploadRequest]()

	// this map will store all the active foreign services
	// that are using the file uplaoding service
	service_map := p_safemap.NewSafeMap[*p_registered_service.Service]()

	// set up router  to different handlers
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

	// TODO: remove this code when testing phase is over
	// study CORS policy
	// set up the proper CORS policy
	cors := handlers.CORS(
		handlers.AllowedOrigins([]string{"http://localhost:3000"}), // your UI origin
		handlers.AllowedMethods([]string{"GET", "POST", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "X-Chunk-Index"}),
		handlers.AllowCredentials(),
	)

	// need to listen to the port tls port
	// need to spawn one more go-routine
	tls_cert, err := load_tls_cert_and_key(filepath.Join(p_global_configs.TLSCERTDST, p_global_configs.TLSCERTNAME), filepath.Join(p_global_configs.TLSCERTDST, p_global_configs.TLSKEYNAME))
	if err != nil {
		cancel()
		fmt.Println("could not load the tls configs ")
		return
	}
	tls_config := &tls.Config{
		Certificates: []tls.Certificate{tls_cert},
		MinVersion:   tls.VersionTLS12,
	}

	go func() {
		tls_srv := &http.Server{
			Addr:      ":8443",
			Handler:   GetRegisterToFileUploadService(ctx, service_map),
			TLSConfig: tls_config,
		}
		log.Fatal(tls_srv.ListenAndServeTLS("", ""))

	}()

	log.Println(" Main Server running on :8000 ")
	http.ListenAndServe(":8000", cors(router))

}

func setUpRouter(safemap *p_safemap.SafeMap[*p_upload_request.UploadRequest], service_map *p_safemap.SafeMap[*p_registered_service.Service]) *mux.Router {
	router := mux.NewRouter()
	router.Handle("/upload/init", getInitUploadSessionHandler(safemap, service_map)).Methods("POST")
	// this route will be secured by tls to ensure the registration process in encrypted
	//router.Handle("/register", GetRegisterToFileUploadService(parent_ctx, service_map))
	return router
}

// take in the safemap
func GetRegisterToFileUploadService(parent_ctx context.Context, service_map *p_safemap.SafeMap[*p_registered_service.Service]) http.Handler {
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

		// after adding to the service_map
		// we will start the service
		// sending the parent context
		service.Start(parent_ctx)

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

func getInitUploadSessionHandler(safemap *p_safemap.SafeMap[*p_upload_request.UploadRequest], service_map *p_safemap.SafeMap[*p_registered_service.Service]) http.Handler {

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
			ServiceID   string `json:"serviceID"`
		}
		// decoding the body
		err := json.NewDecoder(r.Body).Decode(&reqBody)

		// if there were errors whilst decoding, then the request is bad
		if err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		// according to the serviceID get the ptr to the service
		service_ptr, ok := service_map.Get(reqBody.ServiceID)
		if !ok {
			// if the service is not found
			// that means the service hasn't registered to the file upload service
			// need to send the service appropriate message
			return
		}

		// if there are not errors in the request body need to create a NewUploadSession struct
		// I am manually adding the UploadSession
		// create a NewUploadSession
		// TODO need to add some safety measures
		safemap.Add(reqBody.UploadID, &p_upload_request.UploadRequest{
			UploadID:    reqBody.UploadID,
			Service:     service_ptr,
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
