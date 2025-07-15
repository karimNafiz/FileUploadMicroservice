package registered_service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	p_global_configs "github.com/file_upload_microservice/global_configs"
)

// TOOD later on store them in Databases
// also make sure you have some security measures

// / <summary>
// / the service struct will encapsulate another application who wants to use the file upload service.
// / for any application to use the file upload service, they must first register themself to the file upload service
// / when an application registers itself, I will encapsulate them into the service struct
// / </summary>
type Service struct {
	ServiceID string
	Host      string
	Scheme    string
	Port      string
	// when an upload session is complete
	// we need to notify the main service that the job that they requested is complete, failed and stuff
	UploadStatusCallEndPoint         string
	ServiceStatusNotificationChannel chan map[string]map[string]string // im making this into not a ptr, so that not so much pressure is put into heap
}

func NewService(id string, host string, scheme string, port string, upload_status_callback_endpoint string) *Service {
	// TODO: make sure in later versions I had if everything (host, scheme, port, upload_status...endpoint) all follow the appropriate format

	if upload_status_callback_endpoint[0] != '/' {
		upload_status_callback_endpoint = "/" + upload_status_callback_endpoint
	}

	return &Service{
		ServiceID:                        id,
		Host:                             host,
		Port:                             port,
		UploadStatusCallEndPoint:         upload_status_callback_endpoint,
		ServiceStatusNotificationChannel: make(chan map[string]map[string]string, p_global_configs.SERVICESTATUSNOTIFICATIONCHANNELBUFFER),
	}
}

func (s *Service) GetServiceCallBackUrl() string {
	portSegment := ""
	if s.Port != "" {
		portSegment += ":"
	}
	portSegment += s.Port

	return fmt.Sprintf("%s://%s:%s%s", s.Scheme, s.Host, s.Port, s.UploadStatusCallEndPoint)
	// example https://localhost:8000

}

// public function to start the service session
func (s *Service) Start(ctx context.Context) {
	go s.start_service_status_channel_monitor(ctx)
}

func (s *Service) start_service_status_channel_monitor(ctx context.Context) {
	for {

		select {
		case <-ctx.Done():
			// TODO
			// using the callbackURL need to notifiy the foreign service that the file upload service is closed
			return
		case req := <-s.ServiceStatusNotificationChannel:
			// need to make a request using the call back url
			// encode the message and then using the callback url we need to send the encoded message to the foreign service
			switch s.Scheme {
			case p_global_configs.SCHEME_HTTP, p_global_configs.SCHEME_HTTPS:
				// I'm sending the parent go-routines context.
				// If the context is cancelled then it will cascade down into the child go-routine
				go sendHTTP(context.Background(), req["headers"], req["message"], s.GetServiceCallBackUrl())

			}

		}
	}

}

// / <summary>
// / this function will send a http request back to the foreign service, that has registered itself with the file upload service
// / this function assumes proper headers to be passed.
// / moreover, this function assumes messages to be properly tagged with json tags
// / </summary>

// / right now the function lacks any kind of error checking
// / need to implement rettries based on the type of the error
func sendHTTP(ctx context.Context, headers map[string]string, message map[string]string, url string) {
	// first i need to create a context from the parent context
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second) // im setting it to 20
	defer cancel()

	// need to create the payload
	payload, err := json.Marshal(message)
	if err != nil {
		fmt.Println("func: sendHTTP, package: service, error marshalling json")
		return
	}
	// then using that context create create a request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload)) // we have to pass a bytes.Buffer because the function expects a io.Reader

	if err != nil {
		// TODO make sure to check if the error is context related or smth else
		// based on the error try retry strategy
		return
	}
	// add the headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// then create a custom http.Transport
	transport := &http.Transport{
		// e.g.:
		TLSHandshakeTimeout: 5 * time.Second,
		MaxIdleConns:        1,
		IdleConnTimeout:     20 * time.Second,
	}
	// using that we create a http.Client
	client := http.Client{
		Transport: transport,
	}
	resp, err := client.Do(req)
	if err != nil {
		// need to do a switch case based on the error type
		fmt.Println(err.Error())
		return

	}
	defer resp.Body.Close()
	// we can handle the resp.Boyd.Status code and shit

	// then we do our request
}
