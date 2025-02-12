export interface QueryResult {
    id: string;
    query: string;
    executionTime?: number;
    exampleResult?: any[] | null;
    executionResult?: any[] | null;
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
    type: 'user' | 'ai';
    content: string;
    isLoading?: boolean;
    loadingSteps?: LoadingStep[];
    queries?: QueryResult[];
}

export interface LoadingStep {
    text: string;
    done: boolean;
}