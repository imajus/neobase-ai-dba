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
    queries?: {
        id: string;
        query: string;
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
    isLoading: false,
    loadingSteps: [],
    isStreaming: false
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