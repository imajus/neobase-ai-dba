import { Message } from "../components/chat/types";


// Update MessagesResponse to use BackendMessage instead of Message
export interface MessagesResponse {
    success: boolean;
    data: {
        messages: BackendMessage[];
        total: number;
    };
}

// Add interface for backend message format
export interface BackendMessage {
    id: string;
    chat_id: string;
    type: 'user' | 'assistant';
    content: string;
    is_edited: boolean;
    queries?: {
        id: string;
        query: string;
        is_edited: boolean;
        description: string;
        execution_time: number;
        example_execution_time: number;
        can_rollback: boolean;
        is_critical: boolean;
        is_executed: boolean;
        is_rolled_back: boolean;
        error?: {
            code: string;
            message: string;
            details?: string;
        };
        example_result: any[];
        execution_result: any[];
        query_type: string;
        pagination?: {
            total_records_count?: number;
            paginated_query?: string;
        };
        tables: string;
        rollback_query: string;
    }[];
    created_at: string;
}

// Add transform function
export const transformBackendMessage = (msg: BackendMessage): Message => ({
    id: msg.id,
    type: msg.type,
    content: msg.content,
    queries: msg.queries || [],
    is_loading: false,
    loading_steps: [],
    is_streaming: false,
    is_edited: msg.is_edited,
    created_at: msg.created_at
});

// Add interface for the API response
export interface SendMessageResponse {
    success: boolean;
    data: {
        id: string;
        chat_id: string;
        type: 'user' | 'assistant';
        content: string;
        created_at: string;
    };
}