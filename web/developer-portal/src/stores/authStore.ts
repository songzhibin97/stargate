import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { AuthState, User } from '../types';
import { apiService } from '../services/api';

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      isAuthenticated: false,
      user: null,
      token: null,

      login: async (email: string, password: string) => {
        try {
          const response = await apiService.login({ email, password });
          
          // Store token in localStorage
          localStorage.setItem('auth_token', response.token);
          localStorage.setItem('user_data', JSON.stringify(response.user));
          
          set({
            isAuthenticated: true,
            user: response.user,
            token: response.token,
          });
        } catch (error: any) {
          throw new Error(error.message || '登录失败');
        }
      },

      register: async (email: string, name: string, password: string) => {
        try {
          const response = await apiService.register({ email, name, password });
          
          // Store token in localStorage
          localStorage.setItem('auth_token', response.token);
          localStorage.setItem('user_data', JSON.stringify(response.user));
          
          set({
            isAuthenticated: true,
            user: response.user,
            token: response.token,
          });
        } catch (error: any) {
          throw new Error(error.message || '注册失败');
        }
      },

      logout: () => {
        // Clear localStorage
        localStorage.removeItem('auth_token');
        localStorage.removeItem('user_data');
        
        // Clear API service logout (optional, may fail if token is invalid)
        apiService.logout().catch(() => {
          // Ignore logout API errors
        });
        
        set({
          isAuthenticated: false,
          user: null,
          token: null,
        });
      },

      setAuth: (token: string, user: User) => {
        localStorage.setItem('auth_token', token);
        localStorage.setItem('user_data', JSON.stringify(user));
        
        set({
          isAuthenticated: true,
          user,
          token,
        });
      },
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        isAuthenticated: state.isAuthenticated,
        user: state.user,
        token: state.token,
      }),
      onRehydrateStorage: () => (state) => {
        // Validate stored auth data on app load
        if (state?.token && state?.user) {
          const storedToken = localStorage.getItem('auth_token');
          const storedUser = localStorage.getItem('user_data');
          
          if (storedToken && storedUser) {
            try {
              const parsedUser = JSON.parse(storedUser);
              if (parsedUser.id && parsedUser.email) {
                // Auth data is valid
                return;
              }
            } catch (error) {
              // Invalid stored data, clear it
            }
          }
          
          // Clear invalid auth data
          localStorage.removeItem('auth_token');
          localStorage.removeItem('user_data');
          state.isAuthenticated = false;
          state.user = null;
          state.token = null;
        }
      },
    }
  )
);
