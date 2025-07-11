package message

/// <summary>
/// my main purpose for creating this message interface is such that ->
/// the process of upload session and the process of contacting the main service upon the complession or failure ->
/// is decoupled
/// one might raise the question if uploading is the only service that this application will do
/// whats the point of decoupling it.
/// In later versions I plan on introducing more services ->
/// that can be made faster through a chunked approach
/// i currently don't have anything else in mind
/// </summary>

// this interface represents the messages that will be sent back to the main service who have registered themselves
// and have posted some operation to be complete
type Message interface {
	IsSuccessful() bool // need to check if the request operation by the foreign service was successful or not
	GetStatus() string  // return a pointer to a map of type key : string and value: string
	GetMessage() *map[string]string
}
