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
	ServiceStatusNotificationChannel chan map[string]string // im making this into not a ptr, so that not so much pressure is put into heap
}

func NewService(id string, host string, scheme string, port string, upload_status_callback_endpoint string) *Service {
	return &Service{
		ServiceID:                        id,
		Host:                             host,
		Port:                             port,
		UploadStatusCallEndPoint:         upload_status_callback_endpoint,
		ServiceStatusNotificationChannel: make(chan map[string]string, p_global_configs.SERVICESTATUSNOTIFICATIONCHANNELBUFFER),
	}
}

func (s *Service) StartServiceStatusChannelMonitor(ctx context.Context) {
	for {

		select {
		case <-ctx.Done():
			// TODO
			// using the callbackURL need to notifiy the foreign service that the file upload service is closed
			return
		case message <- s.ServiceStatusNotificationChannel:
			// need to make a request using the call back url
			// encode the message and then using the callback url we need to send the encoded message to the foreign service
			switch s.Scheme {
			case p_global_configs.SCHEME_HTTP, p_global_configs.SCHEME_HTTPS:

			}

		}
	}

}

// func sendHTTP(headers map[string]string, message map[string]string) error {
//     // 1) Create a context with a timeout
//     ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//     defer cancel()

//     // (Assuming you have a URL and have marshaled your message:)
//     payload, err := json.Marshal(message)
//     if err != nil {
//         return err
//     }
//     url := "https://your-callback-url/path"

//     // 2) Create a request with that context
//     req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload))
//     if err != nil {
//         return err
//     }
//     // set headers
//     for k, v := range headers {
//         req.Header.Set(k, v)
//     }

//     // 3) Create a custom http.Transport
//     transport := &http.Transport{
//         // e.g.:
//         TLSHandshakeTimeout: 5 * time.Second,
//         MaxIdleConns:        100,
//         IdleConnTimeout:     90 * time.Second,
//     }

//     // 4) Create an http.Client using that transport
//     client := &http.Client{
//         Transport: transport,
//     }

//     // 5) Perform the request
//     resp, err := client.Do(req)
//     if err != nil {
//         return err
//     }
//     defer resp.Body.Close()

//     // handle resp.StatusCode / resp.Body as needed
//     return nil
// }

// need to be serious about contexts and timeouts

// / <summary>
// / this function will send a http request back to the foreign service, that has registered itself with the file upload service
// / this function assumes proper headers to be passed.
// / moreover, this function assumes messages to be properly tagged with json tags
// / </summary>
func sendHTTP(headers map[string]string, message map[string]string, url string) error {
	// first i need to create a context with background timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // im setting it to 20
	defer cancel()

	// need to create the payload
	payload, err := json.Marshal(message)
	if err != nil {
		fmt.Println("func: sendHTTP, package: service, error marshalling json")
		return err
	}
	// then using that context create create a request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload)) // we have to pass a bytes.Buffer because the function expects a io.Reader

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
		return err

	}
	defer resp.Body.Close()
	// we can handle the resp.Boyd.Status code and shit
	return nil

	// then we do our request
}
