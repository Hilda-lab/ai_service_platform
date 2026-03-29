import { API_BASE_URL } from './client'

const BASE_URL = `${API_BASE_URL}/vision`

export type VisionTask = {
  id: number
  user_id: number
  status: 'pending' | 'processing' | 'completed' | 'failed'
  provider: string
  model: string
  prompt: string
  file_name: string
  mime_type: string
  result: string
  error_message: string
  created_at: string
  updated_at: string
}

function buildFormData(file: File, options: { prompt?: string; provider?: string; model?: string }) {
  const form = new FormData()
  form.append('image', file)
  if (options.prompt) form.append('prompt', options.prompt)
  if (options.provider) form.append('provider', options.provider)
  if (options.model) form.append('model', options.model)
  return form
}

export async function recognizeVision(
  token: string,
  file: File,
  options: { prompt?: string; provider?: string; model?: string },
): Promise<VisionTask> {
  const response = await fetch(`${BASE_URL}/recognize`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: buildFormData(file, options),
  })
  const json = (await response.json()) as { data?: VisionTask; message?: string }
  if (!response.ok) throw new Error(json.message || `HTTP ${response.status}`)
  if (!json.data) throw new Error('识别结果为空')
  return json.data
}

export async function submitVisionTask(
  token: string,
  file: File,
  options: { prompt?: string; provider?: string; model?: string },
): Promise<{ task_id: number; status: string }> {
  const response = await fetch(`${BASE_URL}/tasks`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: buildFormData(file, options),
  })
  const json = (await response.json()) as { data?: { task_id: number; status: string }; message?: string }
  if (!response.ok) throw new Error(json.message || `HTTP ${response.status}`)
  if (!json.data) throw new Error('任务响应为空')
  return json.data
}

export async function getVisionTask(token: string, taskId: number): Promise<VisionTask> {
  const response = await fetch(`${BASE_URL}/tasks/${taskId}`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  const json = (await response.json()) as { data?: VisionTask; message?: string }
  if (!response.ok) throw new Error(json.message || `HTTP ${response.status}`)
  if (!json.data) throw new Error('任务不存在')
  return json.data
}
