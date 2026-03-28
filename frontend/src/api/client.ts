const BASE_URL = 'http://localhost:8080/api/v1'

export type ApiResponse<T> = {
  message?: string
  data?: T
  error?: string
}

async function request<T>(path: string, options: RequestInit = {}): Promise<ApiResponse<T>> {
  const response = await fetch(`${BASE_URL}${path}`, {
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
