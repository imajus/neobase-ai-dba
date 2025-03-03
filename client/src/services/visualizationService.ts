import axios from './axiosConfig';
import { AllSuggestionsResponse, TableSuggestionsResponse, VisualizationDataResponse, VisualizationSuggestion } from '../types/visualization';

const API_URL = import.meta.env.VITE_API_URL;

const visualizationService = {
    // Cache for visualization suggestions to avoid redundant API calls
    suggestionsCache: {} as Record<string, { suggestions: VisualizationSuggestion[], timestamp: number }>,
    CACHE_TTL: 300000, // 5 minutes in milliseconds

    async getTableSuggestions(chatId: string, tableName: string): Promise<TableSuggestionsResponse> {
        try {
            // Check cache first
            const cacheKey = `table_${chatId}_${tableName}`;
            const cachedData = this.suggestionsCache[cacheKey];
            
            if (cachedData && (Date.now() - cachedData.timestamp) < this.CACHE_TTL) {
                return {
                    success: true,
                    data: cachedData.suggestions
                };
            }

            const response = await axios.get<TableSuggestionsResponse>(
                `${API_URL}/visualizations/suggestions/table`,
                {
                    params: {
                        chat_id: chatId,
                        table_name: tableName
                    }
                }
            );

            // Cache the results
            if (response.data.success) {
                this.suggestionsCache[cacheKey] = {
                    suggestions: response.data.data,
                    timestamp: Date.now()
                };
            }

            return response.data;
        } catch (error: any) {
            console.error('Get table visualization suggestions error:', error);
            throw new Error(error.response?.data?.error || 'Failed to get visualization suggestions');
        }
    },

    async getAllSuggestions(chatId: string): Promise<AllSuggestionsResponse> {
        try {
            // Check cache first
            const cacheKey = `all_${chatId}`;
            const cachedData = this.suggestionsCache[cacheKey];
            
            if (cachedData && (Date.now() - cachedData.timestamp) < this.CACHE_TTL) {
                return {
                    success: true,
                    data: cachedData.suggestions as any // Cast needed for the structure difference
                };
            }

            const response = await axios.get<AllSuggestionsResponse>(
                `${API_URL}/visualizations/suggestions`,
                {
                    params: {
                        chat_id: chatId
                    }
                }
            );

            // Cache the results (different structure)
            if (response.data.success) {
                this.suggestionsCache[cacheKey] = {
                    suggestions: response.data.data as any, // Cast needed for the structure difference
                    timestamp: Date.now()
                };
            }

            return response.data;
        } catch (error: any) {
            console.error('Get all visualization suggestions error:', error);
            throw new Error(error.response?.data?.error || 'Failed to get visualization suggestions');
        }
    },

    async executeVisualization(
        chatId: string, 
        streamId: string, 
        suggestion: VisualizationSuggestion
    ): Promise<VisualizationDataResponse> {
        try {
            const response = await axios.post<VisualizationDataResponse>(
                `${API_URL}/visualizations/execute`,
                suggestion,
                {
                    params: {
                        chat_id: chatId,
                        stream_id: streamId
                    }
                }
            );

            return response.data;
        } catch (error: any) {
            console.error('Execute visualization error:', error);
            throw new Error(error.response?.data?.error || 'Failed to execute visualization');
        }
    },

    // Clear cache for a specific chat
    clearCache(chatId: string): void {
        Object.keys(this.suggestionsCache).forEach(key => {
            if (key.includes(chatId)) {
                delete this.suggestionsCache[key];
            }
        });
    }
};

export default visualizationService; 