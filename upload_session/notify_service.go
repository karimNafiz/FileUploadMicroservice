package upload_session

// this struct is meant to implement the message interface
type UploadSessionServiceStatus struct {
	IsSuccess bool
	Status    uint   // similar to status codes in http
	Error     string // if there is any error this field will represent that
	Message   string // if not error then we can send generic message through this

}

func NewUploadSessionServiceStatus() *UploadSessionServiceStatus {
	return &UploadSessionServiceStatus{}
}

func (u_status *UploadSessionServiceStatus) IsSuccessful() bool {
	return u_status.IsSuccess
}

func (u_status *UploadSessionServiceStatus) GetStatus() uint {
	return u_status.Status
}

func (u_status *UploadSessionServiceStatus) GetError() string {
	return u_status.Error
}

func (u_status *UploadSessionServiceStatus) GetMessage() string {
	return u_status.Message
}

// type Message interface {
// 	IsSuccessful() bool            // need to check if the request operation by the foreign service was successful or not
// 	GetStatus() *map[string]string // return a pointer to a map of type key : string and value: string
// 	GetError() *map[string]string
// 	GetMessage() *map[string]string
// }
