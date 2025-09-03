import React from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider, theme } from 'antd';
import { QueryClient, QueryClientProvider } from 'react-query';
import { HelmetProvider } from 'react-helmet-async';

import Layout from './components/layout/Layout';
import Dashboard from './pages/Dashboard';
import ApiExplorer from './pages/ApiExplorer';
import ApiTester from './pages/ApiTester';
import ApiDetail from './pages/ApiDetail';
import Profile from './pages/Profile';
import Login from './pages/Login';
import Register from './pages/Register';
import MyApplications from './pages/MyApplications';
import ErrorBoundary from './components/common/ErrorBoundary';

import { useAuthStore } from './stores/authStore';
import { useThemeStore } from './stores/themeStore';

import './App.css';

// Create a client
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 5 * 60 * 1000, // 5 minutes
    },
  },
});

const App: React.FC = () => {
  const { isAuthenticated } = useAuthStore();
  const { isDarkMode } = useThemeStore();

  return (
    <ErrorBoundary>
      <HelmetProvider>
        <QueryClientProvider client={queryClient}>
          <ConfigProvider
            theme={{
              algorithm: isDarkMode ? theme.darkAlgorithm : theme.defaultAlgorithm,
              token: {
                colorPrimary: '#1890ff',
                borderRadius: 6,
              },
            }}
          >
            <Router>
              <div className={`app ${isDarkMode ? 'dark' : 'light'}`}>
                <Routes>
                  <Route path="/login" element={<Login />} />
                  <Route path="/register" element={<Register />} />
                  <Route
                    path="/*"
                    element={
                      isAuthenticated ? (
                        <Layout>
                          <Routes>
                            <Route path="/" element={<Navigate to="/dashboard" replace />} />
                            <Route path="/dashboard" element={<Dashboard />} />
                            <Route path="/applications" element={<MyApplications />} />
                            <Route path="/apis" element={<ApiExplorer />} />
                            <Route path="/apis/:apiId" element={<ApiDetail />} />
                            <Route path="/tester" element={<ApiTester />} />
                            <Route path="/profile" element={<Profile />} />
                            <Route path="*" element={<Navigate to="/dashboard" replace />} />
                          </Routes>
                        </Layout>
                      ) : (
                        <Navigate to="/login" replace />
                      )
                    }
                  />
                </Routes>
              </div>
            </Router>
          </ConfigProvider>
        </QueryClientProvider>
      </HelmetProvider>
    </ErrorBoundary>
  );
};

export default App;
