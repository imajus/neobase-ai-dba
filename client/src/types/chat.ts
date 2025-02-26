// Create a new file for chat types
export interface Connection {
    type: 'postgresql' | 'yugabytedb' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j';
    host: string;
    port: string;
    username: string;
    password?: string;
    database: string;
}

export interface Chat {
    id: string;
    user_id: string;
    connection: Connection;
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