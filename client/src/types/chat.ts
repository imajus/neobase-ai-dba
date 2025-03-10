// Create a new file for chat types
export interface Connection {
    type: 'postgresql' | 'yugabytedb' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j';
    host: string;
    port: string;
    username: string;
    password?: string;
    database: string;
    use_ssl?: boolean;
    ssl_cert_url?: string;
    ssl_key_url?: string;
    ssl_root_cert_url?: string;
}

export interface Chat {
    id: string;
    user_id: string;
    connection: Connection;
    selected_collections?: string; // "ALL" or comma-separated table names
    auto_execute_query?: boolean; // Whether to automatically execute queries when a new message is created
    created_at: string;
    updated_at: string;
}

export interface ChatsResponse {
    success: boolean;
    data: {
        chats: Chat[];
        total: number;
    };
}

export interface CreateChatResponse {
    success: boolean;
    data: Chat;
}

// Table and column information
export interface ColumnInfo {
    name: string;
    type: string;
    is_nullable: boolean;
}

export interface TableInfo {
    name: string;
    columns: ColumnInfo[];
    is_selected: boolean;
}

export interface TablesResponse {
    tables: TableInfo[];
} 