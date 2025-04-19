package dtos

type ConnectionStatusResponse struct {
	IsConnected bool   `json:"is_connected"`
	Type        string `json:"type"`
	Host        string `json:"host"`
	Port        *int   `json:"port"`
	Database    string `json:"database"`
	Username    string `json:"username"`
	IsExampleDB bool   `json:"is_example_db"`
}

type ConnectDBRequest struct {
	StreamID string `json:"stream_id" binding:"required"`
}

type DisconnectDBRequest struct {
	StreamID string `json:"stream_id" binding:"required"`
}
