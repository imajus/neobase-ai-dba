package dtos

type ConnectionStatusResponse struct {
	IsConnected bool   `json:"is_connected"`
	Type        string `json:"type"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Database    string `json:"database"`
	Username    string `json:"username"`
}

type ConnectDBRequest struct {
	ChatID   string `json:"chat_id" binding:"required"`
	StreamID string `json:"stream_id" binding:"required"`
}

type DisconnectDBRequest struct {
	ChatID   string `json:"chat_id" binding:"required"`
	StreamID string `json:"stream_id" binding:"required"`
}
