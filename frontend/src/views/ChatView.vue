<template>
  <main style="padding: 16px; height: calc(100vh - 32px); box-sizing: border-box">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px">
      <h1 style="margin: 0">AI 聊天</h1>
      <div>
        <button style="margin-right: 8px" @click="router.push('/vision')">图像识别</button>
        <button style="margin-right: 8px" @click="router.push('/speech')">语音能力</button>
        <span v-if="email" style="margin-right: 12px">{{ email }}</span>
        <button @click="logout">退出登录</button>
      </div>
    </div>

    <div style="display: grid; grid-template-columns: 260px 1fr; gap: 12px; height: calc(100% - 48px)">
      <aside style="border: 1px solid #ddd; padding: 12px; overflow: auto">
        <button style="width: 100%; margin-bottom: 8px" @click="newSession">+ 新会话</button>
        <div v-for="session in sessions" :key="session.id" style="margin-bottom: 6px">
          <button
            @click="selectSession(session.id)"
            :style="{
              width: '100%',
              textAlign: 'left',
              background: session.id === currentSessionId ? '#f0f0f0' : 'transparent',
            }"
          >
            {{ session.title || `会话 #${session.id}` }}
          </button>
        </div>
      </aside>

      <section style="border: 1px solid #ddd; padding: 12px; display: flex; flex-direction: column; overflow: hidden">
        <div style="display: flex; gap: 8px; margin-bottom: 8px">
          <select v-model="provider" :disabled="loading">
            <option value="openai">OpenAI Relay</option>
            <option value="ollama">Ollama</option>
          </select>
          <input v-model="model" :disabled="loading" placeholder="模型名，例如 gpt-5.1" style="flex: 1" />
          <label>
            <input v-model="useStream" type="checkbox" :disabled="loading" /> 流式
          </label>
          <label>
            <input v-model="useRag" type="checkbox" :disabled="loading" /> 启用RAG
          </label>
        </div>

        <div style="flex: 1; overflow: auto; border: 1px solid #eee; padding: 8px; margin-bottom: 8px">
          <div v-for="(message, index) in messages" :key="index" style="margin-bottom: 8px">
            <strong>{{ message.role === 'user' ? '你' : 'AI' }}：</strong>
            <span>{{ message.content }}</span>
          </div>
        </div>

        <form @submit.prevent="sendMessage" style="display: flex; gap: 8px">
          <input v-model="input" :disabled="loading" placeholder="输入你的问题..." style="flex: 1" />
          <button type="submit" :disabled="loading || !input.trim()">{{ loading ? '发送中...' : '发送' }}</button>
        </form>
        <p v-if="error" style="color: #d93025; margin: 8px 0 0 0">{{ error }}</p>
      </section>
    </div>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getProfile } from '../api/auth'
import { completions, completionsStream, listMessages, listSessions, type ChatMessage, type ChatSession } from '../api/chat'
import { clearToken, getToken } from '../utils/auth'

const router = useRouter()
const email = ref('')
const sessions = ref<ChatSession[]>([])
const messages = ref<Array<{ role: 'user' | 'assistant'; content: string }>>([])
const currentSessionId = ref<number | undefined>()
const provider = ref<'openai' | 'ollama'>('openai')
const model = ref('gpt-5.1')
const useStream = ref(true)
const useRag = ref(false)
const input = ref('')
const loading = ref(false)
const error = ref('')

async function loadSessions(token: string) {
  const resp = await listSessions(token)
  sessions.value = resp.data || []
}

async function selectSession(id: number) {
  const token = getToken()
  if (!token) return
  currentSessionId.value = id
  const resp = await listMessages(token, id)
  const rows = (resp.data || []) as ChatMessage[]
  messages.value = rows.map((item) => ({
    role: item.role,
    content: item.content,
  }))
}

function newSession() {
  currentSessionId.value = undefined
  messages.value = []
}

onMounted(async () => {
  const token = getToken()
  if (!token) {
    await router.push('/login')
    return
  }

  try {
    const profile = await getProfile(token)
    email.value = profile.data?.email || ''
    await loadSessions(token)
  } catch {
    clearToken()
    await router.push('/login')
  }
})

async function sendMessage() {
  const token = getToken()
  if (!token) {
    await router.push('/login')
    return
  }

  const content = input.value.trim()
  if (!content) return

  error.value = ''
  loading.value = true
  messages.value.push({ role: 'user', content })
  input.value = ''

  try {
    if (useStream.value) {
      messages.value.push({ role: 'assistant', content: '' })
      const assistantIndex = messages.value.length - 1

      await completionsStream(
        token,
        {
          session_id: currentSessionId.value,
          provider: provider.value,
          model: model.value,
          message: content,
          use_rag: useRag.value,
        },
        (event) => {
          if (event.type === 'chunk' && event.content) {
            messages.value[assistantIndex].content += event.content
          }
          if (event.type === 'done' && event.session_id) {
            currentSessionId.value = event.session_id
          }
          if (event.type === 'error' && event.message) {
            error.value = event.message
          }
        },
      )
    } else {
      const resp = await completions(token, {
        session_id: currentSessionId.value,
        provider: provider.value,
        model: model.value,
        message: content,
        use_rag: useRag.value,
      })

      if (resp.data?.session_id) {
        currentSessionId.value = resp.data.session_id
      }
      messages.value.push({ role: 'assistant', content: resp.data?.reply || '' })
    }

    await loadSessions(token)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '发送失败'
  } finally {
    loading.value = false
  }
}

async function logout() {
  clearToken()
  await router.push('/login')
}
</script>
