package dtos

type CreateConnectionRequest struct {
	Type     string `json:"type" binding:"required,oneof=postgresql mysql"`
	Host     string `json:"host" binding:"required"`
	Port     string `json:"port" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Database string `json:"database" binding:"required"`
}

type ConnectionResponse struct {
	ID       string `json:"id" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Host     string `json:"host" binding:"required"`
	Port     string `json:"port" binding:"required"`
	Username string `json:"username" binding:"required"`
	Database string `json:"database" binding:"required"`
	// Password not exposed in response
}

type CreateChatRequest struct {
	Connection CreateConnectionRequest `json:"connection" binding:"required"`
}

type UpdateChatRequest struct {
	Connection CreateConnectionRequest `json:"connection" binding:"required"`
}

type ChatResponse struct {
	ID         string             `json:"id"`
	UserID     string             `json:"user_id"`
	Connection ConnectionResponse `json:"connection"`
	CreatedAt  string             `json:"created_at"`
	UpdatedAt  string             `json:"updated_at"`
}

type ChatListResponse struct {
	Chats []ChatResponse `json:"chats"`
	Total int64          `json:"total"`
}
