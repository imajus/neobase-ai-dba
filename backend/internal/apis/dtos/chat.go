package dtos

type CreateConnectionRequest struct {
	Type     string `json:"type" binding:"required,oneof=postgresql yugabytedb mysql clickhouse mongodb redis neo4j"`
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
	Connection          CreateConnectionRequest `json:"connection" binding:"required"`
	SelectedCollections string                  `json:"selected_collections"` // "ALL" or comma-separated table names
}

type ChatResponse struct {
	ID                  string             `json:"id"`
	UserID              string             `json:"user_id"`
	Connection          ConnectionResponse `json:"connection"`
	SelectedCollections string             `json:"selected_collections"`
	CreatedAt           string             `json:"created_at"`
	UpdatedAt           string             `json:"updated_at"`
}

type ChatListResponse struct {
	Chats []ChatResponse `json:"chats"`
	Total int64          `json:"total"`
}

// TableInfo represents a table with its columns
type TableInfo struct {
	Name    string       `json:"name"`
	Columns []ColumnInfo `json:"columns"`
}

// ColumnInfo represents a column in a table
type ColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	IsNullable bool   `json:"is_nullable"`
}

// TablesResponse represents the response for the get tables API
type TablesResponse struct {
	Tables []TableInfo `json:"tables"`
}
