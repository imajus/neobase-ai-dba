package dtos

type StreamResponse struct {
	Event string      `json:"event"` // message, error, db-connected, db-disconnected, sse-connected, response-cancelled
	Data  interface{} `json:"data,omitempty"`
}
