<template>
  <main style="padding: 24px; max-width: 480px; margin: 0 auto">
    <h1>登录 / 注册</h1>

    <form @submit.prevent="onSubmit" style="display: flex; flex-direction: column; gap: 12px">
      <label>
        邮箱
        <input v-model="form.email" type="email" required style="width: 100%; padding: 8px" />
      </label>

      <label>
        密码
        <input v-model="form.password" type="password" minlength="6" required style="width: 100%; padding: 8px" />
      </label>

      <div style="display: flex; gap: 8px">
        <button type="button" @click="mode = 'login'" :disabled="loading">登录模式</button>
        <button type="button" @click="mode = 'register'" :disabled="loading">注册模式</button>
      </div>

      <button type="submit" :disabled="loading">
        {{ loading ? '处理中...' : mode === 'login' ? '登录' : '注册并登录' }}
      </button>
    </form>

    <p v-if="message" style="margin-top: 16px">{{ message }}</p>
  </main>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getProfile, login, register } from '../api/auth'
import { setToken } from '../utils/auth'

const router = useRouter()
const loading = ref(false)
const message = ref('')
const mode = ref<'login' | 'register'>('login')

const form = reactive({
  email: '',
  password: '',
})

async function onSubmit() {
  loading.value = true
  message.value = ''

  try {
    if (mode.value === 'register') {
      await register(form.email, form.password)
      message.value = '注册成功，正在自动登录...'
    }

    const loginResp = await login(form.email, form.password)
    const token = loginResp.data?.token
    if (!token) {
      throw new Error('登录成功但未返回 token')
    }

    setToken(token)
    await getProfile(token)
    message.value = '登录成功，正在跳转...'
    await router.push('/chat')
  } catch (error) {
    message.value = error instanceof Error ? error.message : '请求失败'
  } finally {
    loading.value = false
  }
}
</script>
