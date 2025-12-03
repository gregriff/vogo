package public

// StatusResponse is the http response for GET /status
type StatusResponse struct {
	Friends  []Friend
	Channels []Channel
}
