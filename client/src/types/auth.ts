export interface AuthState {
  isAuthenticated: boolean;
  user: User | null;
}

export interface User {
  id: string;
  username: string;
  created_at: string;
}

export interface LoginFormData {
  userName: string;
  password: string;
}

export interface SignupFormData extends LoginFormData {
  confirmPassword: string;
  userSignupSecret: string;
}

export interface AuthResponse {
  success: boolean;
  data: {
    access_token: string;
    refresh_token: string;
    user: {
      id: string;
      username: string;
      created_at: string;
    };
  };
  error?: string;
}

export interface UserResponse {
  success: boolean;
  data: User;
}