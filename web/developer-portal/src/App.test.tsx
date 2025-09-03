import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import App from './App'


// Mock the App component's internal router by testing individual components
describe('App', () => {
  it('renders without crashing', () => {
    // Since App has its own Router, we test it directly
    render(<App />)

    // Check if the app renders without throwing an error
    expect(document.body).toBeInTheDocument()
  })

  it('contains main application structure', () => {
    render(<App />)

    // The app should render some content - check for the app div
    const appElement = document.querySelector('.app')
    expect(appElement).toBeInTheDocument()
  })
})
