import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from 'react-query'
import Dashboard from './Dashboard'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
})

const DashboardWrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={queryClient}>
    <BrowserRouter>
      {children}
    </BrowserRouter>
  </QueryClientProvider>
)

// Mock the API service
vi.mock('../services/api', () => ({
  getDashboardStats: vi.fn().mockResolvedValue({
    totalApplications: 5,
    totalApiCalls: 1234,
    successRate: 99.5,
    avgResponseTime: 150,
  }),
  getRecentActivity: vi.fn().mockResolvedValue([]),
}))

describe('Dashboard Page', () => {
  it('renders dashboard without crashing', () => {
    render(
      <DashboardWrapper>
        <Dashboard />
      </DashboardWrapper>
    )
    
    // Basic render test
    expect(document.body).toBeInTheDocument()
  })

  it('contains dashboard structure', () => {
    render(
      <DashboardWrapper>
        <Dashboard />
      </DashboardWrapper>
    )
    
    // Check for dashboard elements
    const dashboardContent = document.querySelector('.dashboard, .ant-row, .dashboard-container')
    expect(dashboardContent).toBeInTheDocument()
  })

  it('displays dashboard components', () => {
    render(
      <DashboardWrapper>
        <Dashboard />
      </DashboardWrapper>
    )
    
    // Check for common dashboard elements
    const cards = document.querySelectorAll('.ant-card')
    expect(cards.length).toBeGreaterThan(0)
  })
})
