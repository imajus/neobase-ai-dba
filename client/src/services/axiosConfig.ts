import axios from 'axios';

let failedQueue: any[] = [];

const processQueue = (error: any, token: string | null = null) => {
    failedQueue.forEach(prom => {
        if (error) {
            prom.reject(error);
        } else {
            prom.resolve(token);
        }
    });
    failedQueue = [];
};

// Define API error response type
export interface APIErrorResponse {
    success: boolean;
    error: string;
}

// Create custom error class
export class APIError extends Error {
    constructor(message: string) {
        super(message);
        this.name = 'APIError';
    }
}

// Set default configs for all requests
axios.defaults.headers.common['Content-Type'] = 'application/json';
axios.defaults.headers.common['Accept'] = 'application/json';

// Add request interceptor
axios.interceptors.request.use(
    config => {
        const token = localStorage.getItem('token');
        if (token) {
            config.headers = {
                ...config.headers,
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json',
                'Accept': 'application/json',
            };
        }
        // Add this for all requests
        config.withCredentials = false; // Set to false since we're using Bearer token
        console.log('Starting Request:', {
            url: config.url,
            method: config.method,
            headers: config.headers
        });
        return config;
    },
    error => {
        console.error('API Request Error:', {
            status: error.response?.status,
            data: error.response?.data,
            message: error.message
        });
        const errorMessage = error.response?.data?.error || error.message || 'Request failed';
        return Promise.reject(new APIError(errorMessage));
    }
);

export default axios;