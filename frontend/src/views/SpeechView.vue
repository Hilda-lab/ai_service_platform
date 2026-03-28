<template>
  <main style="padding: 24px; max-width: 900px; margin: 0 auto">
    <h1>语音识别与合成</h1>

    <section style="margin-top: 16px; border: 1px solid #ddd; padding: 12px">
      <h3 style="margin-top: 0">文本转语音 (TTS)</h3>
      <div style="display: grid; gap: 8px">
        <textarea v-model="ttsText" rows="4" placeholder="输入要合成的文本"></textarea>
        <div style="display: flex; gap: 8px">
          <input v-model="ttsModel" placeholder="模型，如 gpt-4o-mini-tts" style="flex: 1" />
          <input v-model="ttsVoice" placeholder="音色，如 alloy" style="width: 180px" />
          <input v-model="ttsLanguage" placeholder="语言，如 zh" style="width: 120px" />
        </div>
        <button @click="runTTS" :disabled="loading || !ttsText.trim()">生成语音</button>
        <audio v-if="audioSrc" :src="audioSrc" controls></audio>
      </div>
    </section>

    <section style="margin-top: 16px; border: 1px solid #ddd; padding: 12px">
      <h3 style="margin-top: 0">语音转文本 (ASR)</h3>
      <div style="display: grid; gap: 8px">
        <input type="file" accept="audio/*" @change="onAudioChange" />
        <div style="display: flex; gap: 8px">
          <input v-model="asrModel" placeholder="模型，如 whisper-1" style="flex: 1" />
          <input v-model="asrLanguage" placeholder="语言，如 zh" style="width: 120px" />
        </div>
        <input v-model="asrPrompt" placeholder="可选提示词" />
        <button @click="runASR" :disabled="loading || !audioFile">识别语音</button>
        <pre v-if="asrText" style="white-space: pre-wrap">{{ asrText }}</pre>
      </div>
    </section>

    <p v-if="error" style="color: #d93025; margin-top: 12px">{{ error }}</p>
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { speechToText, textToSpeech } from '../api/speech'
import { getToken } from '../utils/auth'

const router = useRouter()
const loading = ref(false)
const error = ref('')

const ttsText = ref('')
const ttsModel = ref('gpt-4o-mini-tts')
const ttsVoice = ref('alloy')
const ttsLanguage = ref('zh')
const audioSrc = ref('')

const audioFile = ref<File | null>(null)
const asrModel = ref('whisper-1')
const asrLanguage = ref('zh')
const asrPrompt = ref('')
const asrText = ref('')

function tokenOrRedirect() {
  const token = getToken()
  if (!token) {
    router.push('/login')
    return ''
  }
  return token
}

async function runTTS() {
  const token = tokenOrRedirect()
  if (!token) return
  loading.value = true
  error.value = ''
  try {
    const data = await textToSpeech(token, {
      text: ttsText.value,
      model: ttsModel.value,
      voice: ttsVoice.value,
      language: ttsLanguage.value,
      format: 'mp3',
    })
    audioSrc.value = `data:${data.mimeType};base64,${data.audioBase64}`
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'TTS失败'
  } finally {
    loading.value = false
  }
}

function onAudioChange(event: Event) {
  const input = event.target as HTMLInputElement
  audioFile.value = input.files?.[0] || null
}

async function runASR() {
  if (!audioFile.value) return
  const token = tokenOrRedirect()
  if (!token) return
  loading.value = true
  error.value = ''
  asrText.value = ''
  try {
    const data = await speechToText(token, audioFile.value, {
      model: asrModel.value,
      language: asrLanguage.value,
      prompt: asrPrompt.value,
    })
    asrText.value = data.text
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'ASR失败'
  } finally {
    loading.value = false
  }
}
</script>
