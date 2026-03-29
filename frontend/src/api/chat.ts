import { request } from './client'

export type ChatSession = {
  id: number
  user_id: number
  title: string
  provider: string
  model: string
  created_at: string
  updated_at: string
}

export type ChatMessage = {
  id: number
  session_id: number
  user_id: number
  role: 'user' | 'assistant'
  content: string
  provider: string
  model: string
  created_at: string
}

export type CompletionPayload = {
  session_id?: number
  provider: 'openai'
  model: string
  message: string
  use_rag?: boolean
}

export type CompletionData = {
  session_id: number
  provider: string
  model: string
  reply: string
}

export async function listSessions(token: string) {
  return request<ChatSession[]>('/chat/sessions', {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
}

export async function listMessages(token: string, sessionId: number) {
  return request<ChatMessage[]>(`/chat/sessions/${sessionId}/messages`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
}

export async function completions(token: string, payload: CompletionPayload) {
  return request<CompletionData>('/chat/completions', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify(payload),
  })
}

export async function completionsStream(
  token: string,
  payload: CompletionPayload,
  onEvent: (event: { type: string; content?: string; session_id?: number; message?: string }) => void,
) {
  const response = await fetch('http://localhost:8080/api/v1/chat/completions/stream', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  })

  if (!response.ok || !response.body) {
    const text = await response.text()
    throw new Error(text || `HTTP ${response.status}`)
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { value, done } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const parts = buffer.split('\n\n')
    buffer = parts.pop() || ''

    for (const part of parts) {
      const line = part
        .split('\n')
        .find((row) => row.startsWith('data:'))
      if (!line) continue

      const jsonPayload = line.replace(/^data:\s*/, '')
      try {
        const data = JSON.parse(jsonPayload) as { type: string; content?: string; session_id?: number; message?: string }
        onEvent(data)
      } catch {
        continue
      }
    }
  }
}
