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
    }
};

export default chatService; 