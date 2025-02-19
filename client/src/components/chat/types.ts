export interface QueryResult {
    id: string;
    query: string;
    description: string;
    executionTime?: number;
    exampleResult?: any[] | null;
    executionResult?: any[] | null;
    isExecuted?: boolean;
    isRolledBack?: boolean;
    error?: {
        code: string;
        message: string;
        details?: string;
    };
    isCritical?: boolean;
    canRollback?: boolean;
}

export interface Message {
    id: string;
    type: 'user' | 'assistant';
    content: string;
    isLoading?: boolean;
    loadingSteps?: LoadingStep[];
    queries?: QueryResult[];
    isStreaming?: boolean;
}

export interface LoadingStep {
    text: string;
    done: boolean;
}