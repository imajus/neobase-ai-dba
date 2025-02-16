package dtos

type StreamResponse struct {
	Event string      `json:"event"` // message, error, complete, keepalive
	Data  interface{} `json:"data,omitempty"`
}
