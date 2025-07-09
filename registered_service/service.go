package registered_service

// TOOD later on store them in Databases
// also make sure you have some security measures
type Service struct {
	ServiceID string
	Host      string
	Scheme    string
	Port      string
	// when an upload session is complete
	// we need to notify the main service that the job that they requested is complete, failed and stuff
	UploadStatusCallBackURL string
}

func NewService(id string, host string, scheme string, port string, upload_status_callback_url string) *Service {
	return &Service{
		ServiceID:               id,
		Host:                    host,
		Port:                    port,
		UploadStatusCallBackURL: upload_status_callback_url,
	}
}
