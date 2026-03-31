<template>
  <main style="padding: 24px; max-width: 1200px; margin: 0 auto">
    <div style="display: flex; justify-content: space-between; align-items: center">
      <h1>RAG 文档管理</h1>
      <button @click="router.push('/chat')">返回聊天</button>
    </div>

    <!-- 上传区域 -->
    <section
      @drop="onDrop"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      :style="{
        border: '2px dashed ' + (isDragging ? '#4285f4' : '#ccc'),
        borderRadius: '8px',
        padding: '32px',
        textAlign: 'center',
        marginTop: '16px',
        backgroundColor: isDragging ? '#f0f7ff' : '#f9f9f9',
        cursor: 'pointer',
        transition: 'all 0.3s',
      }"
      @click="fileInput?.click()"
    >
      <input ref="fileInput" type="file" accept=".txt,.md,.pdf,.docx,.xlsx" style="display: none" @change="onFileChange" />
      <div style="font-size: 32px; margin-bottom: 8px">📄</div>
      <h3 style="margin: 0">拖拽或点击上传文档</h3>
      <p style="color: #666; margin: 8px 0 0 0">支持格式：文本 (.txt, .md)、PDF、Word (.docx)、Excel (.xlsx)</p>
    </section>

    <!-- 上传表单 -->
    <section style="display: grid; gap: 12px; margin-top: 16px">
      <div>
        <label style="display: block; margin-bottom: 4px; font-weight: bold">文档标题</label>
        <input
          v-model="documentTitle"
          type="text"
          placeholder="为你的文档起个名字"
          :disabled="uploading"
          style="width: 100%; padding: 8px; border: 1px solid #ccc; border-radius: 4px; box-sizing: border-box"
        />
      </div>

      <div>
        <label style="display: block; margin-bottom: 4px; font-weight: bold">文档内容预览</label>
        <textarea
          v-model="documentContent"
          placeholder="文档内容将显示在此（最多 50KB）"
          :disabled="uploading"
          rows="10"
          style="width: 100%; padding: 12px; border: 1px solid #ddd; border-radius: 4px; box-sizing: border-box; font-family: monospace; font-size: 12px; color: #333; background-color: #fff; line-height: 1.5"
        />
        <span style="font-size: 12px; color: #666; display: block; margin-top: 4px">{{ documentContent.length }} / 51200 字节</span>
      </div>

      <button
        @click="uploadDocument"
        :disabled="uploading || !documentTitle || !documentContent"
        :style="{
          padding: '10px 16px',
          backgroundColor: uploading || !documentTitle || !documentContent ? '#ccc' : '#4285f4',
          color: 'white',
          border: 'none',
          borderRadius: '4px',
          cursor: uploading || !documentTitle || !documentContent ? 'not-allowed' : 'pointer',
          fontSize: '14px',
        }"
      >
        {{ uploading ? '上传中...' : '上传文档' }}
      </button>
    </section>

    <!-- 错误提示 -->
    <p v-if="error" style="color: #d93025; margin-top: 12px">{{ error }}</p>

    <!-- 成功提示 -->
    <p v-if="successMessage" style="color: #0d652d; margin-top: 12px">{{ successMessage }}</p>

    <!-- 文档列表 -->
    <section style="margin-top: 32px">
      <h2 style="margin-top: 0">已上传文档（{{ documents.length }}）</h2>

      <div v-if="loadingDocuments" style="text-align: center; color: #666">加载中...</div>

      <div v-else-if="documents.length === 0" style="text-align: center; color: #666; padding: 32px">
        暂无文档，上传一些文档来开始使用 RAG
      </div>

      <div
        v-else
        style="display: grid; gap: 12px"
      >
        <div
          v-for="doc in documents"
          :key="doc.id"
          style="{
            border: '1px solid #ddd',
            borderRadius: '8px',
            overflow: 'hidden',
            backgroundColor: '#f9f9f9',
          }"
        >
          <!-- 文档头部 -->
          <div
            style="{
              padding: '16px',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              gap: '12px',
            }"
          >
            <div style="flex: 1">
              <h3 style="margin: 0 0 8px 0">{{ doc.title }}</h3>
              <p style="margin: 0; font-size: 12px; color: #666">
                分块数：{{ doc.chunk_count }} | 大小：{{ formatBytes(getContentSizeInBytes(doc.content)) }} | 上传时间：{{ formatDate(doc.created_at) }}
              </p>
            </div>
            <!-- 按钮组 -->
            <div style="display: flex; gap: 8px">
              <button
                @click="toggleViewDocument(doc.id)"
                style="{
                  padding: '6px 12px',
                  backgroundColor: expandedDocId === doc.id ? '#1976d2' : '#0d652d',
                  color: 'white',
                  border: 'none',
                  borderRadius: '4px',
                  cursor: 'pointer',
                  fontSize: '12px',
                }"
              >
                {{ expandedDocId === doc.id ? '隐藏内容' : '查看内容' }}
              </button>
              <button
                @click="deleteDocument(doc.id)"
                :disabled="deleting === doc.id"
                style="{
                  padding: '6px 12px',
                  backgroundColor: deleting === doc.id ? '#ccc' : '#ea4335',
                  color: 'white',
                  border: 'none',
                  borderRadius: '4px',
                  cursor: deleting === doc.id ? 'not-allowed' : 'pointer',
                  fontSize: '12px',
                }"
              >
                {{ deleting === doc.id ? '删除中...' : '删除' }}
              </button>
            </div>
          </div>

          <!-- 展开的内容区域 -->
          <div
            v-if="expandedDocId === doc.id"
            style="{
              borderTop: '1px solid #ddd',
              padding: '16px',
              backgroundColor: '#fff',
              maxHeight: '400px',
              overflowY: 'auto',
            }"
          >
            <div style="font-size: 12px; color: #333; whiteSpace: 'pre-wrap'; wordBreak: 'break-word'; fontFamily: 'monospace'">
              {{ doc.content }}
            </div>
          </div>
        </div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ingestDocument, listDocuments, deleteDocument as deleteRAGDocument, type RAGDocument } from '../api/rag'
import { getToken } from '../utils/auth'

const router = useRouter()
const fileInput = ref<HTMLInputElement | null>(null)
const isDragging = ref(false)
const documentTitle = ref('')
const documentContent = ref('')
const uploading = ref(false)
const deleting = ref<number | null>(null)
const error = ref('')
const successMessage = ref('')
const documents = ref<RAGDocument[]>([])
const loadingDocuments = ref(false)
const expandedDocId = ref<number | null>(null)

function ensureToken() {
  const token = getToken()
  if (!token) {
    router.push('/login')
    return ''
  }
  return token
}

function onDragOver(e: DragEvent) {
  e.preventDefault()
  isDragging.value = true
}

function onDragLeave() {
  isDragging.value = false
}

async function onDrop(e: DragEvent) {
  e.preventDefault()
  isDragging.value = false
  const files = e.dataTransfer?.files
  if (files && files.length > 0) {
    const file = files[0]
    if (isSupportedFileType(file)) {
      await readFile(file)
    } else {
      error.value = '不支持此文件格式，请上传文本、PDF、Word 或 Excel 文件'
    }
  }
}

function onFileChange(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files && input.files.length > 0) {
    const file = input.files[0]
    console.log('选中文件:', file.name, '大小:', file.size, '类型:', file.type)
    error.value = '' // 清空之前的错误
    readFile(file)
  }
}

function isSupportedFileType(file: File): boolean {
  const name = file.name.toLowerCase()
  const supportedExtensions = ['.txt', '.md', '.pdf', '.docx', '.xlsx']
  return supportedExtensions.some(ext => name.endsWith(ext))
}

async function readTextFile(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = (e) => {
      const result = e.target?.result as string
      console.log('FileReader onload - 读取长度:', result?.length)
      if (result && result.includes('\ufffd')) {
        console.warn('检测到乱码字符')
        reject(new Error('文件编码不支持，请使用 UTF-8 编码的纯文本文件'))
      } else {
        resolve(result || '')
      }
    }
    reader.onerror = () => {
      console.error('FileReader error')
      reject(new Error('读取文件出错'))
    }
    console.log('开始 readAsText，文件类型:', file.type)
    reader.readAsText(file, 'UTF-8')
  })
}

async function uploadAndParse(file: File): Promise<string> {
  const token = getToken()
  if (!token) throw new Error('未授权')
  
  const formData = new FormData()
  formData.append('file', file)
  
  console.log('上传二进制文件到后端:', file.name)
  const response = await fetch('http://127.0.0.1:28080/api/v1/rag/parse-file', {
    method: 'POST',
    headers: { 'Authorization': `Bearer ${token}` },
    body: formData
  })
  
  console.log('文件上传响应状态:', response.status)
  const json = (await response.json()) as { data?: { content: string }; message?: string }
  if (!response.ok) {
    console.error('文件上传失败:', json.message)
    throw new Error(json.message || `解析文件失败: ${response.status}`)
  }
  if (!json.data?.content) throw new Error('解析结果为空')
  console.log('文件解析成功，内容长度:', json.data.content.length)
  return json.data.content
}

async function readFile(file: File) {
  error.value = ''
  successMessage.value = ''
  console.log('开始读取文件:', file.name, '类型:', file.type)
  
  if (file.size > 52428800) {
    // 50MB limit
    error.value = '文件过大，请上传小于 50MB 的文件'
    console.warn('文件超过 50MB')
    return
  }
  
  try {
    const fileName = file.name
    let text = ''
    
    if (fileName.endsWith('.pdf') || fileName.endsWith('.docx') || fileName.endsWith('.xlsx')) {
      // 对于二进制文件，发送给后端处理
      successMessage.value = `正在处理 ${fileName}...`
      console.log('切换为后端解析模式，文件:', fileName)
      text = await uploadAndParse(file)
    } else {
      // 对于文本文件，在客户端读取
      console.log('切换为本地解析模式，文件:', fileName)
      text = await readTextFile(file)
    }
    
    console.log('文件读取成功，长度:', text.length, '字节')
    documentTitle.value = fileName.replace(/\.(txt|md|pdf|docx|xlsx)$/i, '')
    documentContent.value = text
    successMessage.value = `✓ 已读取文件：${fileName}（${text.length} 字符）`
    console.log('UI已更新，文档标题:', documentTitle.value)
    setTimeout(() => {
      successMessage.value = ''
    }, 3000)
  } catch (e) {
    const msg = e instanceof Error ? e.message : '未知错误'
    console.error('❌ 读取文件出错:', msg, '堆栈:', e)
    error.value = '❌ 读取文件失败：' + msg
  }
}

async function uploadDocument() {
  if (!documentTitle.value || !documentContent.value) {
    error.value = '请填写标题和内容'
    return
  }

  const token = ensureToken()
  if (!token) return

  uploading.value = true
  error.value = ''
  successMessage.value = ''

  try {
    const result = await ingestDocument(token, documentTitle.value, documentContent.value)
        const chunkCount = Array.isArray(result.chunks) ? result.chunks.length : (result.chunks as any)
        successMessage.value = `成功上传文档"${result.document.title}"，共生成 ${chunkCount} 个分块`
    loadDocuments()

    setTimeout(() => {
      successMessage.value = ''
    }, 5000)
  } catch (e) {
    error.value = '上传失败：' + (e instanceof Error ? e.message : '未知错误')
  } finally {
    uploading.value = false
  }
}

async function loadDocuments() {
  const token = ensureToken()
  if (!token) return

  loadingDocuments.value = true
  error.value = ''

  try {
    documents.value = await listDocuments(token)
  } catch (e) {
    error.value = '加载文档列表失败：' + (e instanceof Error ? e.message : '未知错误')
  } finally {
    loadingDocuments.value = false
  }
}

async function deleteDocument(id: number) {
  const token = ensureToken()
  if (!token) return

  if (!confirm('确定要删除这个文档吗？')) return

  deleting.value = id
  error.value = ''
  successMessage.value = ''

  try {
    await deleteRAGDocument(token, id)
    successMessage.value = '文档已删除'
    loadDocuments()
    setTimeout(() => {
      successMessage.value = ''
    }, 3000)
  } catch (e) {
    error.value = '删除失败：' + (e instanceof Error ? e.message : '未知错误')
  } finally {
    deleting.value = null
  }
}

function toggleViewDocument(id: number) {
  if (expandedDocId.value === id) {
    expandedDocId.value = null
  } else {
    expandedDocId.value = id
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i]
}

function getContentSizeInBytes(content: string): number {
  // 计算字符串的字节长度（UTF-8 编码）
  try {
    return new TextEncoder().encode(content).length
  } catch {
    // 如果失败，回退到字符长度
    return content.length
  }
}

function formatDate(dateStr: string): string {
  try {
    const date = new Date(dateStr)
    return date.toLocaleDateString('zh-CN') + ' ' + date.toLocaleTimeString('zh-CN')
  } catch {
    return dateStr
  }
}

onMounted(() => {
  loadDocuments()
})
</script>
