// API Response Types
export interface ApiResponse<T = any> {
  success: boolean;
  data?: T;
  message?: string;
  error?: string;
  code?: string;
}

// User Types
export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  status: string;
  created_at?: string;
  updated_at?: string;
}

// Authentication Types
export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  name: string;
  password: string;
}

export interface AuthResponse {
  token: string;
  user: User;
  expires_at: string;
}

// Application Types
export interface Application {
  id: string;
  name: string;
  description: string;
  user_id: string;
  api_key: string;
  api_secret?: string;
  status: ApplicationStatus;
  rate_limit: number;
  created_at: string;
  updated_at: string;
}

export enum ApplicationStatus {
  ACTIVE = 'active',
  INACTIVE = 'inactive',
  SUSPENDED = 'suspended'
}

export interface CreateApplicationRequest {
  name: string;
  description: string;
}

export interface UpdateApplicationRequest {
  name?: string;
  description?: string;
}

export interface ApplicationListResponse {
  applications: Application[];
  total: number;
  offset: number;
  limit: number;
}

// API Key Management Types
export interface APIKeyResponse {
  api_key: string;
  message: string;
}

// Theme Types
export interface ThemeState {
  isDarkMode: boolean;
  toggleTheme: () => void;
}

// Auth Store Types
export interface AuthState {
  isAuthenticated: boolean;
  user: User | null;
  token: string | null;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, name: string, password: string) => Promise<void>;
  logout: () => void;
  setAuth: (token: string, user: User) => void;
}

// Form Types
export interface FormErrors {
  [key: string]: string;
}

// Common UI Types
export interface SelectOption {
  label: string;
  value: string;
}

export interface PaginationParams {
  offset: number;
  limit: number;
}

// API Service Types
export interface RequestConfig {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
  headers?: Record<string, string>;
  data?: any;
  params?: Record<string, any>;
}

// Analytics Types
export interface AnalyticsSummaryRequest {
  start_time?: string;
  end_time?: string;
  time_range?: string;
  application_ids?: string[];
  granularity?: string;
  metrics?: string[];
}

export interface AnalyticsSummary {
  total_requests: number;
  successful_requests: number;
  failed_requests: number;
  success_rate: number;
  avg_response_time: number;
  p95_response_time: number;
  p99_response_time: number;
  total_data_transferred: number;
  unique_endpoints: number;
  most_active_application?: ApplicationSummary;
}

export interface ApplicationSummary {
  application_id: string;
  application_name: string;
  request_count: number;
}

export interface TimeSeriesData {
  timestamp: string;
  request_count: number;
  success_count: number;
  error_count: number;
  avg_response_time: number;
  data_transferred: number;
}

export interface ApplicationAnalytics {
  application_id: string;
  application_name: string;
  total_requests: number;
  successful_requests: number;
  failed_requests: number;
  success_rate: number;
  avg_response_time: number;
  p95_response_time: number;
  data_transferred: number;
  top_endpoints: EndpointStats[];
  error_breakdown: ErrorStats[];
}

export interface EndpointStats {
  path: string;
  method: string;
  request_count: number;
  avg_response_time: number;
  success_rate: number;
}

export interface ErrorStats {
  status_code: number;
  count: number;
  percentage: number;
  common_message?: string;
}

export interface ResponseMetadata {
  start_time: string;
  end_time: string;
  granularity: string;
  application_count: number;
  data_freshness: string;
  query_duration: number;
}

export interface AnalyticsSummaryResponse {
  summary: AnalyticsSummary;
  time_series: TimeSeriesData[];
  applications: ApplicationAnalytics[];
  metadata: ResponseMetadata;
}

// Dashboard Types
export interface DashboardState {
  timeRange: string;
  selectedApplications: string[];
  granularity: string;
  autoRefresh: boolean;
  refreshInterval: number;
}

export interface ChartData {
  labels: string[];
  datasets: ChartDataset[];
}

export interface ChartDataset {
  label: string;
  data: number[];
  backgroundColor?: string | string[];
  borderColor?: string;
  borderWidth?: number;
  fill?: boolean;
}
