<script setup lang="ts">
import { reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { toErrorMessage } from "../services/response";

const router = useRouter();
const auth = useAuthStore();

const loading = ref(false);
const errorMessage = ref("");

const form = reactive({
  username: "",
  password: "",
});

async function submit(): Promise<void> {
  if (!form.username.trim() || !form.password) {
    errorMessage.value = "请输入用户名和密码";
    return;
  }

  loading.value = true;
  errorMessage.value = "";

  try {
    await auth.login({
      username: form.username.trim(),
      password: form.password,
    });
    await router.replace({ name: "chat" });
  } catch (error) {
    errorMessage.value = toErrorMessage(error);
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <main class="auth-page">
    <section class="auth-card login-card">
      <header class="auth-header">
        <div class="auth-logo">IM</div>
        <h1>欢迎回来</h1>
        <p class="auth-subtitle">登录后即刻连接你的实时消息</p>
      </header>

      <form class="auth-form" @submit.prevent="submit">
        <label>
          <span>用户名</span>
          <input v-model="form.username" autocomplete="username" placeholder="请输入用户名" />
        </label>

        <label>
          <span>密码</span>
          <input v-model="form.password" type="password" autocomplete="current-password" placeholder="请输入密码" />
        </label>

        <p v-if="errorMessage" class="auth-error">{{ errorMessage }}</p>

        <button class="auth-submit" :disabled="loading" type="submit">
          {{ loading ? "登录中..." : "登录" }}
        </button>
      </form>

      <footer class="auth-footer">
        还没有账号？
        <RouterLink to="/register">立即注册</RouterLink>
      </footer>
    </section>
  </main>
</template>

<style scoped>
.login-card {
  width: min(440px, 96vw);
}

.auth-header {
  margin-bottom: 28px;
  text-align: center;
}

.auth-logo {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 52px;
  height: 52px;
  border-radius: 16px;
  background: linear-gradient(135deg, #f472b6, #d4507a);
  color: #fff;
  font-size: 18px;
  font-weight: 800;
  letter-spacing: 0.05em;
  margin-bottom: 16px;
  box-shadow: 0 6px 20px -4px rgba(212, 80, 122, 0.35);
}

h1 {
  margin: 0 0 6px;
  font-size: 28px;
  font-weight: 700;
  color: var(--text);
}

.auth-subtitle {
  margin: 0;
  color: var(--text-soft);
  font-size: 14px;
}

.auth-form {
  display: grid;
  gap: 16px;
}

label {
  display: grid;
  gap: 6px;
}

label span {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-soft);
}

input {
  width: 100%;
  padding: 11px 14px;
  border-radius: var(--radius-sm);
  border: 1.5px solid var(--border);
  background: var(--surface-strong);
  color: var(--text);
  font-size: 14px;
  transition: border-color 0.2s, box-shadow 0.2s;
}

input:focus {
  outline: none;
  border-color: #e88aab;
  box-shadow: 0 0 0 3px rgba(212, 80, 122, 0.1);
}

input::placeholder {
  color: #c4a3ae;
}

.auth-submit {
  margin-top: 4px;
  height: 44px;
  border-radius: var(--radius-sm);
  border: none;
  background: linear-gradient(135deg, #f472b6, #d4507a);
  color: #fff;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.2s, transform 0.1s;
  box-shadow: 0 4px 14px -2px rgba(212, 80, 122, 0.3);
}

.auth-submit:hover:not(:disabled) {
  opacity: 0.92;
}

.auth-submit:active:not(:disabled) {
  transform: scale(0.99);
}

.auth-submit:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.auth-error {
  margin: 0;
  padding: 8px 12px;
  border-radius: 8px;
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #b91c1c;
  font-size: 13px;
}

.auth-footer {
  margin-top: 20px;
  text-align: center;
  color: var(--text-soft);
  font-size: 14px;
}

.auth-footer a {
  color: var(--primary);
  font-weight: 600;
  text-decoration: none;
  transition: color 0.15s;
}

.auth-footer a:hover {
  color: var(--primary-strong);
}
</style>
