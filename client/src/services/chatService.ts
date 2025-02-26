import { Chat, Connection } from '../types/chat';
import { ExecuteQueryResponse, MessagesResponse, SendMessageResponse } from '../types/messages';
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
    async editChat(chatId: string, connection: Connection): Promise<Chat> {
        try {
            const response = await axios.put<CreateChatResponse>(
                `${API_URL}/chats/${chatId}`,
                {
                    connection: connection
                },
                {
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );

            if (!response.data.success) {
                throw new Error('Failed to edit chat');
            }

            return response.data.data;
        } catch (error: any) {
            console.error('Edit chat error:', error);
            throw new Error(error.response?.data?.error || 'Failed to edit chat');
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
            return false;
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

    async disconnectFromConnection(chatId: string, streamId: string): Promise<void> {
        try {
            const response = await axios.post(`${API_URL}/chats/${chatId}/disconnect`, {
                stream_id: streamId
            }, {
                withCredentials: true,
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('token')}`
                }
            });
            return response.data.success;
        } catch (error: any) {
            console.error('Disconnect from connection error:', error);
            throw new Error(error.response?.data?.error || 'Failed to disconnect from connection');
        }
    },

    async getMessages(chatId: string, page: number, perPage: number): Promise<MessagesResponse> {
        try {
            const response = await axios.get<MessagesResponse>(
                `${import.meta.env.VITE_API_URL}/chats/${chatId}/messages?page=${page}&page_size=${perPage}`,
                {
                    withCredentials: true,
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            return response.data;
        } catch (error: any) {
            console.error('Get messages error:', error);
            throw new Error(error.response?.data?.error || 'Failed to get messages');
        }
    },
    async sendMessage(chatId: string, messageId: string, streamId: string, content: string): Promise<SendMessageResponse> {
        try {
            const response = await axios.post<SendMessageResponse>(
                `${API_URL}/chats/${chatId}/messages`,
                {
                    message_id: messageId,
                    stream_id: streamId,
                    content: content
                },
                {
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            return response.data
        } catch (error: any) {
            console.error('Send message error:', error);
            throw new Error(error.response?.data?.error || 'Failed to send message');
        }
    },

    async executeQuery(chatId: string, messageId: string, queryId: string, streamId: string, controller: AbortController): Promise<ExecuteQueryResponse | undefined> {
        try {
            const response = await axios.post<ExecuteQueryResponse>(
                `${API_URL}/chats/${chatId}/queries/execute`,
                {
                    message_id: messageId,
                    query_id: queryId,
                    stream_id: streamId
                },
                {
                    signal: controller.signal,
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            console.log('chatService executeQuery response', response);
            return response.data;
        } catch (error: any) {
            if (error.name === 'CanceledError' || error.name === 'AbortError') {
                return undefined;
            }
            console.error('Execute query error:', error);
            throw new Error(error.response?.data?.error || 'Failed to execute query');
        }
    },

    async rollbackQuery(chatId: string, messageId: string, queryId: string, streamId: string, controller: AbortController): Promise<ExecuteQueryResponse | undefined> {
        try {
            const response = await axios.post<ExecuteQueryResponse>(`${API_URL}/chats/${chatId}/queries/rollback`, {
                message_id: messageId,
                query_id: queryId,
                stream_id: streamId
            },
                {
                    signal: controller.signal,
                    withCredentials: true,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': `Bearer ${localStorage.getItem('token')}`
                    }
                }
            );
            return response.data;
        } catch (error: any) {
            if (error.name === 'CanceledError' || error.name === 'AbortError') {
                return undefined;
            }
            console.error('Rollback query error:', error);
            throw new Error(error.response?.data?.error || 'Failed to rollback query');
        }
    },

    async refreshSchema(chatId: string): Promise<boolean> {
        try {
            const response = await axios.post(`${API_URL}/chats/${chatId}/refresh-schema`, {
                withCredentials: true,
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('token')}`
                }
            });
            return response.data.success;
        } catch (error: any) {
            console.error('Refresh schema error:', error);
            throw new Error(error.response?.data?.error || 'Failed to refresh schema');
        }
    },

    async editQuery(
        chatId: string,
        messageId: string,
        queryId: string,
        query: string
    ): Promise<{ success: boolean; data?: any }> {
        try {
            const response = await axios.patch(
                `${import.meta.env.VITE_API_URL}/chats/${chatId}/queries/edit`,
                {
                    "message_id": messageId,
                    "query_id": queryId,
                    "query": query
                },
                {
                    withCredentials: true,
                }
            );
            return { success: true, data: response.data };
        } catch (error: any) {
            throw error.response?.data?.error || 'Failed to edit query';
        }
    }
};

export default chatService; 