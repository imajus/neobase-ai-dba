import { LoginFormData, SignupFormData } from '../types/auth';
import axios, { APIError } from './axiosConfig';

const API_URL = import.meta.env.VITE_API_URL;

export interface AuthResponse {
    success: boolean;
    data: {
        access_token: string;
        refresh_token: string;
        user: {
            id: string;
            username: string;
        };
    };
    error?: string;
}

const authService = {
    async login(data: LoginFormData): Promise<AuthResponse> {
        try {
            const response = await axios.post(`${API_URL}/auth/login`, {
                username: data.userName,
                password: data.password,
            });
            if (response.data.data?.access_token) {
                localStorage.setItem('token', response.data.data.access_token);
                localStorage.setItem('refresh_token', response.data.data.refresh_token);
            }
            return response.data;
        } catch (error: any) {
            console.log("login error", error);
            if (error.response?.data?.error) {
                throw new Error(error.response.data.error);
            }
            throw new Error(error.message || 'Login failed');
        }
    },

    async signup(data: SignupFormData): Promise<AuthResponse> {
        try {
            const response = await axios.post(`${API_URL}/auth/signup`, {
                username: data.userName,
                password: data.password,
                user_signup_secret: data.userSignupSecret
            });
            if (response.data.data?.access_token) {
                localStorage.setItem('token', response.data.data.access_token);
                localStorage.setItem('refresh_token', response.data.data.refresh_token);
            }
            return response.data;
        } catch (error: any) {
            if (error.response?.data?.error) {
                throw new Error(error.response.data.error);
            }
            throw new Error(error.message || 'Signup failed');
        }
    },

    async checkAuth(): Promise<boolean> {
        try {
            const token = localStorage.getItem('token');
            if (!token) return false;

            const response = await axios.get(`${API_URL}/chats`, {
                withCredentials: true,
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'Content-Type': 'application/json'
                },
                validateStatus: (status) => {
                    return status === 200 || status === 204;
                }
            });
            console.log("checkAuth response", response);
            return true;
        } catch (error: any) {
            console.error('Auth check error:', error);
            localStorage.removeItem('token');
            localStorage.removeItem('refresh_token');
            if (error.response?.data?.error) {
                throw new APIError(error.response.data.error);
            }
            return false;
        }
    },

    async refreshToken(): Promise<string | null> {
        try {
            const refreshToken = localStorage.getItem('refresh_token');
            if (!refreshToken) return null;

            const response = await axios.post(`${API_URL}/auth/refresh-token`, {}, {
                headers: {
                    Authorization: `Bearer ${refreshToken}`
                }
            });

            if (response.data.data?.access_token) {
                localStorage.setItem('token', response.data.data.access_token);
                return response.data.data.access_token;
            }
            return null;
        } catch (error) {
            console.error('Token refresh failed:', error);
            return null;
        }
    },

    logout() {
        localStorage.removeItem('token');
        localStorage.removeItem('refresh_token');
    }
};

export default authService; 