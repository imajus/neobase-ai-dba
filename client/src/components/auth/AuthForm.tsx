import { AlertCircle, Boxes, KeyRound, Loader, Lock, UserRound } from 'lucide-react';
import React, { useState } from 'react';
import { LoginFormData, SignupFormData } from '../../types/auth';

interface AuthFormProps {
  onLogin: (data: LoginFormData) => Promise<void>;
  onSignup: (data: SignupFormData) => Promise<void>;
}

interface FormErrors {
  userName?: string;
  password?: string;
  confirmPassword?: string;
  userSignupSecret?: string;
}

export default function AuthForm({ onLogin, onSignup }: AuthFormProps) {
  const [isLogin, setIsLogin] = useState(true);
  const [errors, setErrors] = useState<FormErrors>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isLoading, setIsLoading] = useState(false);
  const [formData, setFormData] = useState<SignupFormData>({
    userName: '',
    password: '',
    confirmPassword: '',
    userSignupSecret: ''
  });
  const [formError, setFormError] = useState<string | null>(null);
  const Environment = import.meta.env.VITE_ENVIRONMENT;
  const validateUserName = (userName: string) => {
    if (!userName) return 'Username is required';
    if (userName.length < 3) return 'Username must be at least 3 characters';
    return '';
  };

  const validatePassword = (password: string) => {
    if (!password) return 'Password is required';
    if (password.length < 6) {
      return 'Password must be at least 6 characters';
    }
    return '';
  };

  const validateUserSignupSecret = (userSignupSecret: string) => {
    if (!userSignupSecret) return 'User signup secret is required';
    return '';
  };
  const validateForm = () => {
    console.log(Environment);
    const newErrors: FormErrors = {};

    const userNameError = validateUserName(formData.userName);
    if (userNameError) newErrors.userName = userNameError;

    const passwordError = validatePassword(formData.password);
    if (passwordError) newErrors.password = passwordError;

    const userSignupSecretError = validateUserSignupSecret(formData.userSignupSecret);
    if (!isLogin && Environment != "DEVELOPMENT" && userSignupSecretError) newErrors.userSignupSecret = userSignupSecretError;

    if (!isLogin && formData.password !== formData.confirmPassword) {
      newErrors.confirmPassword = 'Passwords do not match';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrors({});
    setFormError(null);

    if (!validateForm()) return;

    setIsLoading(true);
    try {
      if (isLogin) {
        const { userName, password } = formData;
        await onLogin({ userName, password });
      } else {
        await onSignup(formData);
      }
    } catch (error: any) {
      setFormError(error.message);
    } finally {
      setIsLoading(false);
    }
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: value
    }));

    if (touched[name]) {
      if (name === 'userName') {
        const error = validateUserName(value);
        setErrors(prev => ({ ...prev, userName: error }));
      } else if (name === 'password') {
        const error = validatePassword(value);
        setErrors(prev => ({ ...prev, password: error }));
      } else if (name === 'confirmPassword') {
        setErrors(prev => ({
          ...prev,
          confirmPassword: value !== formData.password ? 'Passwords do not match' : ''
        }));
      }
    }
  };

  const handleBlur = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setTouched(prev => ({ ...prev, [name]: true }));

    if (name === 'userName') {
      const error = validateUserName(value);
      setErrors(prev => ({ ...prev, userName: error }));
    } else if (name === 'password') {
      const error = validatePassword(value);
      setErrors(prev => ({ ...prev, password: error }));
    } else if (name === 'confirmPassword') {
      setErrors(prev => ({
        ...prev,
        confirmPassword: value !== formData.password ? 'Passwords do not match' : ''
      }));
    }
  };

  return (
    <div className="min-h-screen bg-[#FFDB58]/20 flex items-center justify-center p-4">
      <div className="w-full max-w-md neo-border bg-white p-4 md:p-8">
        <h1 className="text-2xl md:text-3xl font-bold text-center mb-2 flex items-center justify-center gap-2">
          <Boxes className="w-10 h-10" />
          NeoBase
        </h1>
        <p className="text-gray-600 text-center mb-8">
          {isLogin ? 'Welcome back to the NeoBase!' : 'Create your account to start using NeoBase'}
        </p>

        {formError && (
          <div className="mb-6 p-4 bg-red-50 border-2 border-red-500 rounded-lg">
            <div className="flex items-center gap-2 text-red-600">
              <AlertCircle className="w-5 h-5" />
              <p className="font-medium">{formError}</p>
            </div>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-6">
          <div>
            <div className="relative">
              <UserRound className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
              <input
                type="text"
                name="userName"
                placeholder="Username"
                value={formData.userName}
                onChange={handleChange}
                onBlur={handleBlur}
                className={`neo-input pl-12 w-full ${errors.userName && touched.userName ? 'border-neo-error' : ''
                  }`}
                required
              />
            </div>
            {errors.userName && touched.userName && (
              <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                <AlertCircle className="w-4 h-4" />
                <span>{errors.userName}</span>
              </div>
            )}
          </div>

          {!isLogin && Environment !== "DEVELOPMENT" && (
            <div>
              <div className="relative">
                <Lock className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
                <input
                  type="text"
                  name="userSignupSecret"
                  placeholder="User Signup Secret"
                  value={formData.userSignupSecret}
                  onChange={handleChange}
                  onBlur={handleBlur}
                  className={`neo-input pl-12 w-full ${errors.userSignupSecret && touched.userSignupSecret ? 'border-neo-error' : ''
                    }`}
                  required
                />
              </div>
              <p className="text-gray-500 text-sm mt-2">
                Required to signup new user, ask admin for the secret
              </p>
              {errors.userSignupSecret && touched.userSignupSecret && (
                <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                  <AlertCircle className="w-4 h-4" />
                  <span>{errors.userSignupSecret}</span>
                </div>
              )}
            </div>
          )}

          <div>
            <div className="relative">
              <KeyRound className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
              <input
                type="password"
                name="password"
                placeholder="Password"
                value={formData.password}
                onChange={handleChange}
                onBlur={handleBlur}
                className={`neo-input pl-12 w-full ${errors.password && touched.password ? 'border-neo-error' : ''
                  }`}
                required
              />
            </div>
            {errors.password && touched.password && (
              <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                <AlertCircle className="w-4 h-4" />
                <span>{errors.password}</span>
              </div>
            )}
          </div>

          {!isLogin && (
            <div>
              <div className="relative">
                <KeyRound className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
                <input
                  type="password"
                  name="confirmPassword"
                  placeholder="Confirm Password"
                  value={formData.confirmPassword}
                  onChange={handleChange}
                  onBlur={handleBlur}
                  className={`neo-input pl-12 w-full ${errors.confirmPassword && touched.confirmPassword ? 'border-neo-error' : ''
                    }`}
                  required
                />
              </div>
              {errors.confirmPassword && touched.confirmPassword && (
                <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                  <AlertCircle className="w-4 h-4" />
                  <span>{errors.confirmPassword}</span>
                </div>
              )}
            </div>
          )}

          <button
            type="submit"
            className="neo-button w-full relative"
            disabled={isLoading}
          >
            {isLoading ? (
              <div className="flex items-center justify-center">
                <Loader className="w-4 h-4 animate-spin text-gray-400 mr-2" />
                {isLogin ? 'Logging in...' : 'Signing up...'}
              </div>
            ) : (
              isLogin ? 'Login' : 'Sign Up'
            )}
          </button>
          <div className="my-2" />
          <button
            type="button"
            onClick={() => setIsLogin(!isLogin)}
            className="neo-button-secondary w-full"
            disabled={isLoading}
          >
            {isLogin ? 'Switch to Sign Up' : 'Switch to Login'}
          </button>
        </form>
      </div>
    </div>
  );
}