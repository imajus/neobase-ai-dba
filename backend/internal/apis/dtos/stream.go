package dtos

type StreamResponse struct {
	Event string      `json:"event"` // ai-response, ai-response-step, ai-response-error, db-connected, db-disconnected, sse-connected, response-cancelled, query-results, rollback-executed, rollback-query-failed
	Data  interface{} `json:"data,omitempty"`
}
