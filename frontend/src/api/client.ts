const API_ORIGIN = (import.meta.env.VITE_API_BASE_URL || 'http://127.0.0.1:28080').replace(/\/$/, '')
export const API_BASE_URL = `${API_ORIGIN}/api/v1`

export type ApiResponse<T> = {
  message?: string
  data?: T
  error?: string
}

async function request<T>(path: string, options: RequestInit = {}): Promise<ApiResponse<T>> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    },
    ...options,
  })

  const json = (await response.json()) as ApiResponse<T>
  if (!response.ok) {
    throw new Error(json.message || json.error || `HTTP ${response.status}`)
  }
  return json
}

export { request }
