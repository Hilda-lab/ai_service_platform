import { API_BASE_URL } from './client'

const BASE_URL = `${API_BASE_URL}/rag`

export type RAGDocument = {
  id: number
  user_id: number
  title: string
  content: string
  chunk_count: number
  created_at: string
  updated_at: string
}

export type RAGChunk = {
  id: number
  document_id: number
  content: string
  embedding: number[]
  created_at: string
}

export type RetrieveResult = {
  chunk_id: number
  document_id: number
  title: string
  content: string
  similarity_score: number
}

export type RetrieveMetrics = {
  query: string
  top_k: number
  corpus_size: number
  matched_count: number
  max_score: number
  average_score: number
  min_score_limit: number
}

export async function ingestDocument(token: string, title: string, content: string): Promise<{ document: RAGDocument; chunks: RAGChunk[] }> {
  const response = await fetch(`${BASE_URL}/documents`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ title, content }),
  })
  const json = (await response.json()) as { data?: { document: RAGDocument; chunks: RAGChunk[] }; message?: string }
  if (!response.ok) throw new Error(json.message || `HTTP ${response.status}`)
  if (!json.data) throw new Error('摄取结果为空')
  return json.data
}

export async function listDocuments(token: string): Promise<RAGDocument[]> {
  const response = await fetch(`${BASE_URL}/documents`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  const json = (await response.json()) as { data?: RAGDocument[]; message?: string }
  if (!response.ok) throw new Error(json.message || `HTTP ${response.status}`)
  if (!json.data) throw new Error('文档列表为空')
  return json.data
}

export async function retrieveDocuments(
  token: string,
  query: string,
  topK: number = 5,
): Promise<{ results: RetrieveResult[]; metrics: RetrieveMetrics }> {
  const response = await fetch(`${BASE_URL}/retrieve`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ query, top_k: topK }),
  })
  const json = (await response.json()) as { data?: RetrieveResult[]; metrics?: RetrieveMetrics; message?: string }
  if (!response.ok) throw new Error(json.message || `HTTP ${response.status}`)
  if (!json.data) throw new Error('检索结果为空')
  return { results: json.data, metrics: json.metrics || {} as RetrieveMetrics }
}

export async function deleteDocument(token: string, documentID: number): Promise<void> {
  const response = await fetch(`${BASE_URL}/documents/${documentID}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
  })
  const json = (await response.json()) as { message?: string }
  if (!response.ok) throw new Error(json.message || `HTTP ${response.status}`)
}

export async function getPerformanceStats(token: string): Promise<PerformanceStats> {
  const response = await fetch(`${BASE_URL}/stats`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  const json = (await response.json()) as { data?: PerformanceStats; message?: string }
  if (!response.ok) throw new Error(json.message || `HTTP ${response.status}`)
  if (!json.data) throw new Error('stats data is empty')
  return json.data
}
