import { Chat, Connection } from '../types/chat';
import axios from './axiosConfig';

const API_URL = import.meta.env.VITE_API_URL;

interface CreateChatResponse {
    success: boolean;
    data: Chat;
}

const chatService = {
    async createChat(connection: Connection): Promise<Chat> {
        try {
            const response = await axios.post<CreateChatResponse>(`${API_URL}/chats`, {
                connection
            });

            if (!response.data.success) {
                throw new Error('Failed to create chat');
            }

            return response.data.data;
        } catch (error: any) {
            console.error('Create chat error:', error);
            throw new Error(error.response?.data?.error || 'Failed to create chat');
        }
    },

    async deleteChat(chatId: string): Promise<void> {
        try {
            const response = await axios.delete(`${API_URL}/chats/${chatId}`);

            if (!response.data.success && response.status !== 200) {
                throw new Error('Failed to delete chat');
            }
        } catch (error: any) {
            console.error('Delete chat error:', error);
            throw new Error(error.response?.data?.error || 'Failed to delete chat');
        }
    },

    async checkConnectionStatus(chatId: string): Promise<boolean> {
        try {
            const response = await axios.get(`${API_URL}/chats/${chatId}/connection-status`);
            return response.data.success;
        } catch (error: any) {
            console.error('Check connection status error:', error);
            throw new Error(error.response?.data?.error || 'Failed to check connection status');
        }
    },

    async connectToConnection(chatId: string, streamId: string): Promise<void> {
        try {
            const response = await axios.post(`${API_URL}/chats/${chatId}/connect`, { stream_id: streamId });
            return response.data.success;
        } catch (error: any) {
            console.error('Connect to connection error:', error);
            throw new Error(error.response?.data?.error || 'Failed to connect to connection');
        }
    },

    async disconnectFromConnection(chatId: string): Promise<void> {
        try {
            const response = await axios.post(`${API_URL}/chats/${chatId}/disconnect`);
            return response.data.success;
        } catch (error: any) {
            console.error('Disconnect from connection error:', error);
            throw new Error(error.response?.data?.error || 'Failed to disconnect from connection');
        }
    },

    async executeQuery(chatId: string, messageId: string, queryId: string, streamId: string): Promise<void> {
        try {
            await axios.post(`${API_URL}/chats/${chatId}/queries/execute`, {
                message_id: messageId,
                query_id: queryId,
                stream_id: streamId
            },
                {
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
        } catch (error: any) {
            console.error('Execute query error:', error);
            throw new Error(error.response?.data?.error || 'Failed to execute query');
        }
    },

    async rollbackQuery(chatId: string, messageId: string, queryId: string, streamId: string): Promise<void> {
        try {
            await axios.post(`${API_URL}/chats/${chatId}/queries/rollback`, {
                message_id: messageId,
                query_id: queryId,
                stream_id: streamId
            },
                {
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
        } catch (error: any) {
            console.error('Rollback query error:', error);
            throw new Error(error.response?.data?.error || 'Failed to rollback query');
        }
    }
};

export default chatService; 