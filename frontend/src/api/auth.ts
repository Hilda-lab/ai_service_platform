import { request } from './client'

export type UserProfile = {
  id: number
  email: string
}

export type LoginData = {
  token: string
  user: UserProfile
}

export async function register(email: string, password: string) {
  return request<{ id: number; email: string }>('/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export async function login(email: string, password: string) {
  return request<LoginData>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export async function getProfile(token: string) {
  return request<UserProfile>('/auth/profile', {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })
}
