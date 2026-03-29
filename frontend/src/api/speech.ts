import { API_BASE_URL } from './client'

const BASE_URL = `${API_BASE_URL}/speech`

export async function textToSpeech(
  token: string,
  payload: { text: string; model?: string; voice?: string; format?: string; language?: string },
) {
  const response = await fetch(`${BASE_URL}/tts`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  })

  const json = (await response.json()) as { data?: { audioBase64: string; mimeType: string }; message?: string }
  if (!response.ok) throw new Error(json.message || `HTTP ${response.status}`)
  if (!json.data) throw new Error('TTS返回为空')
  return json.data
}

export async function speechToText(
  token: string,
  file: File,
  payload: { model?: string; language?: string; prompt?: string },
) {
  const form = new FormData()
  form.append('audio', file)
  if (payload.model) form.append('model', payload.model)
  if (payload.language) form.append('language', payload.language)
  if (payload.prompt) form.append('prompt', payload.prompt)

  const response = await fetch(`${BASE_URL}/asr`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  })

  const json = (await response.json()) as { data?: { text: string }; message?: string }
  if (!response.ok) throw new Error(json.message || `HTTP ${response.status}`)
  if (!json.data) throw new Error('ASR返回为空')
  return json.data
}
