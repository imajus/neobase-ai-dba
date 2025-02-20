export interface StreamResponse {
    event: 'ai-response' | 'ai-response-step' | 'ai-response-error' | 'db-connected' |
    'db-disconnected' | 'sse-connected' | 'response-cancelled' | 'query-executed' |
    'rollback-executed' | 'query-execution-failed' | 'rollback-query-failed';
    data?: any;
} 