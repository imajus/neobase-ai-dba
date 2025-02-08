import React, { useState } from 'react';
import { LoginFormData, SignupFormData } from '../../types/auth';
import { KeyRound, Mail, UserPlus, LogIn, Boxes, AlertCircle } from 'lucide-react';

interface AuthFormProps {
  onLogin: (data: LoginFormData) => void;
  onSignup: (data: SignupFormData) => void;
}

interface FormErrors {
  email?: string;
  password?: string;
  confirmPassword?: string;
}

export default function AuthForm({ onLogin, onSignup }: AuthFormProps) {
  const [isLogin, setIsLogin] = useState(true);
  const [errors, setErrors] = useState<FormErrors>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [formData, setFormData] = useState<SignupFormData>({
    email: '',
    password: '',
    confirmPassword: '',
  });

  const validateEmail = (email: string) => {
    if (!email) return 'Email is required';
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      return 'Please enter a valid email address';
    }
    return '';
  };

  const validatePassword = (password: string) => {
    if (!password) return 'Password is required';
    if (password.length < 6) {
      return 'Password must be at least 6 characters';
    }
    return '';
  };

  const validateForm = () => {
    const newErrors: FormErrors = {};
    
    const emailError = validateEmail(formData.email);
    if (emailError) newErrors.email = emailError;

    const passwordError = validatePassword(formData.password);
    if (passwordError) newErrors.password = passwordError;

    if (!isLogin && formData.password !== formData.confirmPassword) {
      newErrors.confirmPassword = 'Passwords do not match';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setErrors({});

    if (!validateForm()) return;

    if (isLogin) {
      const { email, password } = formData;
      onLogin({ email, password });
    } else {
      onSignup(formData);
    }
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: value
    }));

    if (touched[name]) {
      if (name === 'email') {
        const error = validateEmail(value);
        setErrors(prev => ({ ...prev, email: error }));
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

    if (name === 'email') {
      const error = validateEmail(value);
      setErrors(prev => ({ ...prev, email: error }));
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
          {isLogin ? 'Welcome back!' : 'Create your account'}
        </p>

        <form onSubmit={handleSubmit} className="space-y-6">
          <div>
            <div className="relative">
              <Mail className="absolute left-4 top-1/2 transform -translate-y-1/2 text-gray-500" />
              <input
                type="email"
                name="email"
                placeholder="Email"
                value={formData.email}
                onChange={handleChange}
                onBlur={handleBlur}
                className={`neo-input pl-12 w-full ${
                  errors.email && touched.email ? 'border-neo-error' : ''
                }`}
                required
              />
            </div>
            {errors.email && touched.email && (
              <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                <AlertCircle className="w-4 h-4" />
                <span>{errors.email}</span>
              </div>
            )}
          </div>

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
                className={`neo-input pl-12 w-full ${
                  errors.password && touched.password ? 'border-neo-error' : ''
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
                  className={`neo-input pl-12 w-full ${
                    errors.confirmPassword && touched.confirmPassword ? 'border-neo-error' : ''
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

          <button type="submit" className="neo-button w-full">
            {isLogin ? 'Login' : 'Sign Up'}
          </button>
          <div className="my-2" />
          <button
            type="button"
            onClick={() => setIsLogin(!isLogin)}
            className="neo-button-secondary w-full"
          >
            {isLogin ? 'Switch to Sign Up' : 'Switch to Login'}
          </button>
        </form>
      </div>
    </div>
  );
}