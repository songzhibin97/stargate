import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from 'react-query'
import Login from './Login'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
})

const LoginWrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={queryClient}>
    <BrowserRouter>
      {children}
    </BrowserRouter>
  </QueryClientProvider>
)

// Mock the API service
vi.mock('../services/api', () => ({
  login: vi.fn(),
}))

describe('Login Page', () => {
  it('renders login form', () => {
    render(
      <LoginWrapper>
        <Login />
      </LoginWrapper>
    )
    
    // Check for login form elements
    expect(document.querySelector('form')).toBeInTheDocument()
  })

  it('renders without crashing', () => {
    render(
      <LoginWrapper>
        <Login />
      </LoginWrapper>
    )
    
    // Basic render test
    expect(document.body).toBeInTheDocument()
  })

  it('contains login page structure', () => {
    render(
      <LoginWrapper>
        <Login />
      </LoginWrapper>
    )
    
    // Check for basic page structure
    const pageContent = document.querySelector('.login-page, .ant-form, form')
    expect(pageContent).toBeInTheDocument()
  })
})
