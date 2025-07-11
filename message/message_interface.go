package message

type Message interface {
	IsSuccessful() bool            // need to check if the request operation by the foreign service was successful or not
	GetStatus() *map[string]string // return a pointer to a map of type key : string and value: string
	GetError() *map[string]string
	GetMessage() *map[string]string
}
