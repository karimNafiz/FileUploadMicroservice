package upload_session

type ServiceOperationStatus struct {
	IsSuccess bool
	Status    uint   // similar to status codes in http
	Error     string // if there is any error this field will represent that
	Message   string // if not error then we can send generic message through this

}
