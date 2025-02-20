export interface QueryResult {
    id: string;
    query: string;
    description: string;
    execution_time?: number | null;
    example_execution_time?: number | null;
    example_result?: any[] | null;
    execution_result?: any[] | null;
    is_executed?: boolean;
    is_rolled_back?: boolean;
    error?: {
        code: string;
        message: string;
        details?: string;
    };
    is_critical?: boolean;
    can_rollback?: boolean;
    is_streaming?: boolean;
}

export interface Message {
    id: string;
    type: 'user' | 'assistant';
    content: string;
    is_loading?: boolean;
    loading_steps?: LoadingStep[];
    queries?: QueryResult[];
    is_streaming?: boolean;
}

export interface LoadingStep {
    text: string;
    done: boolean;
}