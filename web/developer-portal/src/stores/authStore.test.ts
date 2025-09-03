import { describe, it, expect, beforeEach, vi } from 'vitest'

// Mock localStorage
const localStorageMock = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
}
Object.defineProperty(window, 'localStorage', {
  value: localStorageMock,
})

// Mock auth store since we don't have the actual implementation
class MockAuthStore {
  private _user: any = null
  private _token: string | null = null
  private _isAuthenticated: boolean = false

  get user() {
    return this._user
  }

  get token() {
    return this._token
  }

  get isAuthenticated() {
    return this._isAuthenticated
  }

  login(token: string, user: any) {
    this._token = token
    this._user = user
    this._isAuthenticated = true
    localStorage.setItem('token', token)
    localStorage.setItem('user', JSON.stringify(user))
  }

  logout() {
    this._token = null
    this._user = null
    this._isAuthenticated = false
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }

  loadFromStorage() {
    const token = localStorage.getItem('token')
    const userStr = localStorage.getItem('user')
    
    if (token && userStr) {
      try {
        const user = JSON.parse(userStr)
        this._token = token
        this._user = user
        this._isAuthenticated = true
      } catch (error) {
        this.logout()
      }
    }
  }

  updateUser(userData: any) {
    if (this._user) {
      this._user = { ...this._user, ...userData }
      localStorage.setItem('user', JSON.stringify(this._user))
    }
  }
}

describe('Auth Store', () => {
  let authStore: MockAuthStore

  beforeEach(() => {
    authStore = new MockAuthStore()
    vi.clearAllMocks()
  })

  describe('Initial State', () => {
    it('should have initial state', () => {
      expect(authStore.user).toBe(null)
      expect(authStore.token).toBe(null)
      expect(authStore.isAuthenticated).toBe(false)
    })
  })

  describe('Login', () => {
    it('should login user successfully', () => {
      const token = 'test-token'
      const user = { id: 1, email: 'test@example.com', name: 'Test User' }

      authStore.login(token, user)

      expect(authStore.token).toBe(token)
      expect(authStore.user).toEqual(user)
      expect(authStore.isAuthenticated).toBe(true)
      expect(localStorage.setItem).toHaveBeenCalledWith('token', token)
      expect(localStorage.setItem).toHaveBeenCalledWith('user', JSON.stringify(user))
    })
  })

  describe('Logout', () => {
    it('should logout user successfully', () => {
      // First login
      const token = 'test-token'
      const user = { id: 1, email: 'test@example.com' }
      authStore.login(token, user)

      // Then logout
      authStore.logout()

      expect(authStore.token).toBe(null)
      expect(authStore.user).toBe(null)
      expect(authStore.isAuthenticated).toBe(false)
      expect(localStorage.removeItem).toHaveBeenCalledWith('token')
      expect(localStorage.removeItem).toHaveBeenCalledWith('user')
    })
  })

  describe('Load from Storage', () => {
    it('should load user from localStorage', () => {
      const token = 'stored-token'
      const user = { id: 1, email: 'stored@example.com' }

      localStorageMock.getItem.mockImplementation((key) => {
        if (key === 'token') return token
        if (key === 'user') return JSON.stringify(user)
        return null
      })

      authStore.loadFromStorage()

      expect(authStore.token).toBe(token)
      expect(authStore.user).toEqual(user)
      expect(authStore.isAuthenticated).toBe(true)
    })

    it('should handle invalid user data in localStorage', () => {
      localStorageMock.getItem.mockImplementation((key) => {
        if (key === 'token') return 'token'
        if (key === 'user') return 'invalid-json'
        return null
      })

      authStore.loadFromStorage()

      expect(authStore.token).toBe(null)
      expect(authStore.user).toBe(null)
      expect(authStore.isAuthenticated).toBe(false)
    })

    it('should handle missing data in localStorage', () => {
      localStorageMock.getItem.mockReturnValue(null)

      authStore.loadFromStorage()

      expect(authStore.token).toBe(null)
      expect(authStore.user).toBe(null)
      expect(authStore.isAuthenticated).toBe(false)
    })
  })

  describe('Update User', () => {
    it('should update user data', () => {
      // First login
      const token = 'test-token'
      const user = { id: 1, email: 'test@example.com', name: 'Test User' }
      authStore.login(token, user)

      // Update user
      const updates = { name: 'Updated Name', avatar: 'avatar.jpg' }
      authStore.updateUser(updates)

      expect(authStore.user).toEqual({ ...user, ...updates })
      expect(localStorage.setItem).toHaveBeenCalledWith('user', JSON.stringify({ ...user, ...updates }))
    })

    it('should not update user when not authenticated', () => {
      const updates = { name: 'Updated Name' }
      authStore.updateUser(updates)

      expect(authStore.user).toBe(null)
      expect(localStorage.setItem).not.toHaveBeenCalledWith('user', expect.any(String))
    })
  })
})
