<template>
  <main style="padding: 24px; max-width: 980px; margin: 0 auto">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px">
      <h1 style="margin: 0">MCP 工具测试面板</h1>
      <button @click="router.push('/chat')">返回聊天</button>
    </div>

    <section style="border: 1px solid #ddd; padding: 12px; margin-bottom: 12px">
      <div style="display: flex; gap: 8px; flex-wrap: wrap">
        <button @click="connect" :disabled="connecting || connected">{{ connecting ? '连接中...' : '连接' }}</button>
        <button @click="disconnect" :disabled="!connected">断开连接</button>
        <button @click="sendPing" :disabled="!connected">Ping</button>
        <button @click="listTools" :disabled="!connected">列出工具</button>
      </div>
      <p style="margin: 8px 0 0 0">连接状态：{{ connected ? '已连接' : '未连接' }}</p>
      <p v-if="error" style="color: #d93025; margin: 8px 0 0 0">{{ error }}</p>
    </section>

    <section style="border: 1px solid #ddd; padding: 12px; margin-bottom: 12px">
      <h3 style="margin-top: 0">时间工具</h3>
      <div style="display: flex; gap: 8px; align-items: center; flex-wrap: wrap">
        <input v-model="timezone" placeholder="Asia/Shanghai" style="width: 220px" />
        <button @click="callDatetime" :disabled="!connected">调用 get_datetime</button>
      </div>
    </section>

    <section style="border: 1px solid #ddd; padding: 12px; margin-bottom: 12px">
      <h3 style="margin-top: 0">天气工具</h3>
      <div style="display: flex; gap: 8px; align-items: center; flex-wrap: wrap">
        <input v-model="city" placeholder="城市名，例如 Shanghai" style="width: 220px" />
        <input v-model="weatherTimezone" placeholder="时区，例如 Asia/Shanghai" style="width: 220px" />
        <button @click="callWeather" :disabled="!connected">调用 query_weather</button>
      </div>
      <p style="margin: 8px 0 0 0; color: #666">可只填城市；不填城市默认 Shanghai。</p>
    </section>

    <section style="border: 1px solid #ddd; padding: 12px">
      <h3 style="margin-top: 0">返回日志</h3>
      <pre style="white-space: pre-wrap; max-height: 420px; overflow: auto">{{ logs.join('\n\n') }}</pre>
    </section>
  </main>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useRouter } from 'vue-router'
import { API_BASE_URL } from '../api/client'
import { getToken } from '../utils/auth'

const router = useRouter()
const connected = ref(false)
const connecting = ref(false)
const error = ref('')
const logs = ref<string[]>([])
const timezone = ref('Asia/Shanghai')
const city = ref('Shanghai')
const weatherTimezone = ref('Asia/Shanghai')

let ws: WebSocket | null = null

function appendLog(title: string, payload: unknown) {
  const serialized = typeof payload === 'string' ? payload : JSON.stringify(payload, null, 2)
  logs.value.push(`[${new Date().toLocaleTimeString()}] ${title}\n${serialized}`)
}

function websocketUrl(token: string) {
  const wsBase = API_BASE_URL.replace(/^http/, 'ws').replace(/\/api\/v1$/, '')
  return `${wsBase}/api/v1/mcp/ws?token=${encodeURIComponent(token)}`
}

function connect() {
  error.value = ''
  const token = getToken()
  if (!token) {
    router.push('/login')
    return
  }

  if (ws && connected.value) {
    return
  }

  connecting.value = true
  const url = websocketUrl(token)
  ws = new WebSocket(url)

  ws.onopen = () => {
    connected.value = true
    connecting.value = false
    appendLog('连接成功', { url: url.replace(/token=[^&]+/, 'token=***') })
  }

  ws.onmessage = (event) => {
    try {
      appendLog('收到消息', JSON.parse(event.data))
    } catch {
      appendLog('收到消息', event.data)
    }
  }

  ws.onerror = () => {
    error.value = 'WebSocket 连接异常'
  }

  ws.onclose = () => {
    connected.value = false
    connecting.value = false
    ws = null
    appendLog('连接关闭', 'closed')
  }
}

function disconnect() {
  if (ws) {
    ws.close()
  }
}

function sendRequest(payload: Record<string, unknown>) {
  if (!ws || !connected.value) {
    error.value = '请先连接 MCP'
    return
  }
  const raw = JSON.stringify(payload)
  ws.send(raw)
  appendLog('发送请求', payload)
}

function sendPing() {
  sendRequest({ jsonrpc: '2.0', id: `ping-${Date.now()}`, method: 'ping', params: {} })
}

function listTools() {
  sendRequest({ jsonrpc: '2.0', id: `tools-${Date.now()}`, method: 'tool.list', params: {} })
}

function callDatetime() {
  sendRequest({
    jsonrpc: '2.0',
    id: `datetime-${Date.now()}`,
    method: 'tool.call',
    params: {
      tool_name: 'get_datetime',
      args: {
        timezone: timezone.value || 'Asia/Shanghai',
      },
    },
  })
}

function callWeather() {
  sendRequest({
    jsonrpc: '2.0',
    id: `weather-${Date.now()}`,
    method: 'tool.call',
    params: {
      tool_name: 'query_weather',
      args: {
        city: city.value || 'Shanghai',
        timezone: weatherTimezone.value || 'Asia/Shanghai',
      },
    },
  })
}

onBeforeUnmount(() => {
  if (ws) {
    ws.close()
  }
})
</script>
