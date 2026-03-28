<template>
  <main style="padding: 24px; max-width: 900px; margin: 0 auto">
    <h1>图像识别</h1>

    <div style="display: grid; gap: 10px; margin-top: 16px">
      <input type="file" accept="image/*" @change="onFileChange" />
      <input v-model="prompt" placeholder="可选提示词，例如：请提取图片中的关键信息" />
      <div style="display: flex; gap: 8px">
        <select v-model="provider">
          <option value="openai">OpenAI Relay</option>
        </select>
        <input v-model="model" placeholder="模型名，如 gpt-4.1-mini" style="flex: 1" />
      </div>
      <div style="display: flex; gap: 8px">
        <button @click="runSync" :disabled="loading || !file">同步识别</button>
        <button @click="runAsync" :disabled="loading || !file">异步提交</button>
        <button @click="pollTask" :disabled="loading || !taskId">查询任务</button>
      </div>
    </div>

    <p v-if="taskId" style="margin-top: 12px">当前任务ID：{{ taskId }}</p>
    <p v-if="status">任务状态：{{ status }}</p>
    <p v-if="error" style="color: #d93025">{{ error }}</p>

    <section v-if="result" style="margin-top: 16px; border: 1px solid #ddd; padding: 12px">
      <h3 style="margin-top: 0">识别结果</h3>
      <pre style="white-space: pre-wrap">{{ result }}</pre>
    </section>
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { getVisionTask, recognizeVision, submitVisionTask } from '../api/vision'
import { getToken } from '../utils/auth'

const router = useRouter()
const file = ref<File | null>(null)
const prompt = ref('')
const provider = ref('openai')
const model = ref('gpt-4.1-mini')
const taskId = ref<number | null>(null)
const status = ref('')
const result = ref('')
const error = ref('')
const loading = ref(false)

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  file.value = input.files?.[0] || null
}

function ensureToken() {
  const token = getToken()
  if (!token) {
    router.push('/login')
    return ''
  }
  return token
}

async function runSync() {
  if (!file.value) return
  const token = ensureToken()
  if (!token) return
  loading.value = true
  error.value = ''
  try {
    const task = await recognizeVision(token, file.value, {
      prompt: prompt.value,
      provider: provider.value,
      model: model.value,
    })
    taskId.value = task.id
    status.value = task.status
    result.value = task.result || ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : '识别失败'
  } finally {
    loading.value = false
  }
}

async function runAsync() {
  if (!file.value) return
  const token = ensureToken()
  if (!token) return
  loading.value = true
  error.value = ''
  result.value = ''
  try {
    const resp = await submitVisionTask(token, file.value, {
      prompt: prompt.value,
      provider: provider.value,
      model: model.value,
    })
    taskId.value = resp.task_id
    status.value = resp.status
  } catch (e) {
    error.value = e instanceof Error ? e.message : '提交失败'
  } finally {
    loading.value = false
  }
}

async function pollTask() {
  if (!taskId.value) return
  const token = ensureToken()
  if (!token) return
  loading.value = true
  error.value = ''
  try {
    const task = await getVisionTask(token, taskId.value)
    status.value = task.status
    if (task.status === 'completed') {
      result.value = task.result || ''
    }
    if (task.status === 'failed') {
      error.value = task.error_message || '任务失败'
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : '查询失败'
  } finally {
    loading.value = false
  }
}
</script>
