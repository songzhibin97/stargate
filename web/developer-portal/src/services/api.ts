import axios, { AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios';
import {
  AuthResponse,
  LoginRequest,
  RegisterRequest,
  Application,
  CreateApplicationRequest,
  UpdateApplicationRequest,
  ApplicationListResponse,
  APIKeyResponse,
  PaginationParams,
  AnalyticsSummaryRequest,
  AnalyticsSummaryResponse
} from '../types';

class ApiService {
  private api: AxiosInstance;
  private baseURL: string;

  constructor() {
    this.baseURL = import.meta.env.VITE_API_BASE_URL || '/api';
    
    this.api = axios.create({
      baseURL: this.baseURL,
      timeout: 10000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Request interceptor to add auth token
    this.api.interceptors.request.use(
      (config) => {
        const token = localStorage.getItem('auth_token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
      },
      (error) => {
        return Promise.reject(error);
      }
    );

    // Response interceptor to handle errors
    this.api.interceptors.response.use(
      (response) => response,
      (error) => {
        if (error.response?.status === 401) {
          // Clear auth data on 401
          localStorage.removeItem('auth_token');
          localStorage.removeItem('user_data');
          window.location.href = '/login';
        }
        return Promise.reject(error);
      }
    );
  }

  // Generic request method
  private async request<T>(config: AxiosRequestConfig): Promise<T> {
    try {
      const response: AxiosResponse<T> = await this.api.request(config);
      return response.data;
    } catch (error: any) {
      if (error.response?.data) {
        throw new Error(error.response.data.message || error.response.data.error || 'Request failed');
      }
      throw new Error(error.message || 'Network error');
    }
  }

  // Authentication APIs
  async login(credentials: LoginRequest): Promise<AuthResponse> {
    return this.request<AuthResponse>({
      method: 'POST',
      url: '/login',
      data: credentials,
    });
  }

  async register(userData: RegisterRequest): Promise<AuthResponse> {
    return this.request<AuthResponse>({
      method: 'POST',
      url: '/register',
      data: userData,
    });
  }

  async logout(): Promise<void> {
    return this.request<void>({
      method: 'POST',
      url: '/v1/auth/logout',
    });
  }

  // Application Management APIs
  async getApplications(params?: PaginationParams): Promise<ApplicationListResponse> {
    return this.request<ApplicationListResponse>({
      method: 'GET',
      url: '/applications',
      params,
    });
  }

  async getApplication(id: string): Promise<Application> {
    return this.request<Application>({
      method: 'GET',
      url: `/applications/${id}`,
    });
  }

  async createApplication(data: CreateApplicationRequest): Promise<Application> {
    return this.request<Application>({
      method: 'POST',
      url: '/applications/create',
      data,
    });
  }

  async updateApplication(id: string, data: UpdateApplicationRequest): Promise<Application> {
    return this.request<Application>({
      method: 'PUT',
      url: `/applications/${id}`,
      data,
    });
  }

  async deleteApplication(id: string): Promise<void> {
    return this.request<void>({
      method: 'DELETE',
      url: `/applications/${id}`,
    });
  }

  async regenerateAPIKey(id: string): Promise<APIKeyResponse> {
    return this.request<APIKeyResponse>({
      method: 'POST',
      url: `/applications/${id}/regenerate-key`,
    });
  }

  // Analytics APIs
  async getAnalyticsSummary(params?: AnalyticsSummaryRequest): Promise<AnalyticsSummaryResponse> {
    return this.request<AnalyticsSummaryResponse>({
      method: 'GET',
      url: '/v1/analytics/summary',
      params,
    });
  }

  async getAnalyticsHealth(): Promise<{ status: string }> {
    return this.request<{ status: string }>({
      method: 'GET',
      url: '/v1/analytics/health',
    });
  }

  // Health check
  async healthCheck(): Promise<{ status: string }> {
    return this.request<{ status: string }>({
      method: 'GET',
      url: '/v1/health',
    });
  }
}

export const apiService = new ApiService();
export default apiService;
