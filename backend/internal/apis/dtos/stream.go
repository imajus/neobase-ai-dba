package dtos

type StreamResponse struct {
	Event string      `json:"event"` // ai-response, ai-response-step, ai-response-error, db-connected, db-disconnected, sse-connected, response-cancelled
	Data  interface{} `json:"data,omitempty"`
}
