// Create a new file for chat types
export interface Connection {
    id: string;
    type: string;
    host: string;
    port: string;
    username: string;
    database: string;
    password?: string;
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