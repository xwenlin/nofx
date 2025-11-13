/**
 * HTTP Client with unified error handling and 401 interception
 *
 * Features:
 * - Unified fetch wrapper
 * - Automatic 401 token expiration handling
 * - Auth state cleanup on unauthorized
 * - Automatic redirect to login page
 */

import { toast } from 'sonner'

const BASE_URL = import.meta.env.BASE_URL || '/'

function buildFullPath(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const normalizedBase = BASE_URL.endsWith('/')
    ? BASE_URL.slice(0, -1)
    : BASE_URL
  if (normalizedBase === '') {
    return normalizedPath
  }
  if (normalizedBase === '/') {
    return normalizedPath
  }
  return `${normalizedBase}${normalizedPath}`
}

const LOGIN_PATH = buildFullPath('/login')
const ROOT_PATH = buildFullPath('/')

function isLoginPath(pathname: string): boolean {
  const path = pathname.split('?')[0].replace(/\/$/, '')
  const login = LOGIN_PATH.replace(/\/$/, '')
  return path === login
}

export class HttpClient {
  // Singleton flag to prevent duplicate 401 handling
  private static isHandling401 = false

  /**
   * Reset 401 handling flag (call after successful login)
   */
  public reset401Flag(): void {
    HttpClient.isHandling401 = false
  }

  /**
   * Show login required notification to user
   */
  private showLoginRequiredNotification(): void {
    toast.warning('登录已过期，请先登录', { duration: 1800 })
  }

  /**
   * Response interceptor - handles common HTTP errors
   *
   * @param response - Fetch Response object
   * @returns Response if successful
   * @throws Error with user-friendly message
   */
  private async handleResponse(response: Response): Promise<Response> {
    // Handle 401 Unauthorized - Token expired or invalid
    if (response.status === 401) {
      // Prevent duplicate 401 handling when multiple API calls fail simultaneously
      if (HttpClient.isHandling401) {
        throw new Error('登录已过期，请重新登录')
      }

      // Set flag to prevent race conditions
      HttpClient.isHandling401 = true

      // Clean up local storage
      localStorage.removeItem('auth_token')
      localStorage.removeItem('auth_user')

      // Notify global listeners (AuthContext will react to this)
      window.dispatchEvent(new Event('unauthorized'))

      // Show user-friendly notification (only once)
      this.showLoginRequiredNotification()

      // Delay redirect to let user see the notification
      setTimeout(() => {
        const currentPath = window.location.pathname
        if (!isLoginPath(currentPath)) {
          const returnUrl = currentPath + window.location.search
          const normalizedReturnPath = returnUrl.split('?')[0]
          const sanitizedReturnPath =
            normalizedReturnPath === '/'
              ? '/'
              : normalizedReturnPath.replace(/\/$/, '')
          const sanitizedRoot =
            ROOT_PATH === '/' ? '/' : ROOT_PATH.replace(/\/$/, '')
          if (
            !isLoginPath(normalizedReturnPath) &&
            sanitizedReturnPath !== sanitizedRoot
          ) {
            sessionStorage.setItem('returnUrl', returnUrl)
          }

          window.location.href = LOGIN_PATH
        }
        // Note: No need to reset flag since we're redirecting
      }, 1500) // 1.5秒延迟,让用户看到提示

      throw new Error('登录已过期，请重新登录')
    }

    // Handle other common errors
    if (response.status === 403) {
      throw new Error('没有权限访问此资源')
    }

    if (response.status === 404) {
      throw new Error('请求的资源不存在')
    }

    if (response.status >= 500) {
      throw new Error('服务器错误，请稍后重试')
    }

    return response
  }

  /**
   * GET request
   */
  async get(url: string, headers?: Record<string, string>): Promise<Response> {
    const response = await fetch(url, {
      method: 'GET',
      headers,
    })
    return this.handleResponse(response)
  }

  /**
   * POST request
   */
  async post(
    url: string,
    body?: any,
    headers?: Record<string, string>
  ): Promise<Response> {
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...headers,
      },
      body: body ? JSON.stringify(body) : undefined,
    })
    return this.handleResponse(response)
  }

  /**
   * PUT request
   */
  async put(
    url: string,
    body?: any,
    headers?: Record<string, string>
  ): Promise<Response> {
    const response = await fetch(url, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        ...headers,
      },
      body: body ? JSON.stringify(body) : undefined,
    })
    return this.handleResponse(response)
  }

  /**
   * DELETE request
   */
  async delete(
    url: string,
    headers?: Record<string, string>
  ): Promise<Response> {
    const response = await fetch(url, {
      method: 'DELETE',
      headers,
    })
    return this.handleResponse(response)
  }

  /**
   * Generic request method for custom configurations
   */
  async request(url: string, options: RequestInit = {}): Promise<Response> {
    const response = await fetch(url, options)
    return this.handleResponse(response)
  }
}

// Export singleton instance
export const httpClient = new HttpClient()

// Export helper function to reset 401 flag
export const reset401Flag = () => httpClient.reset401Flag()
