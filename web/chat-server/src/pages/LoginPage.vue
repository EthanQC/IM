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
        <p class="auth-kicker">IM</p>
        <h1>欢迎回来</h1>
        <p class="auth-subtitle">使用账号登录并连接实时消息服务</p>
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
  width: min(460px, 96vw);
}

.auth-header {
  margin-bottom: 22px;
}

.auth-kicker {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.28em;
  text-transform: uppercase;
  color: var(--primary-strong);
}

h1 {
  margin: 8px 0 6px;
  font-size: 34px;
}

.auth-subtitle {
  margin: 0;
  color: var(--text-soft);
}

.auth-form {
  display: grid;
  gap: 14px;
}

label {
  display: grid;
  gap: 8px;
}

label span {
  font-size: 13px;
  color: var(--text-soft);
}

input {
  width: 100%;
  padding: 12px 14px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: var(--surface-strong);
  color: var(--text);
}

input:focus {
  outline: 2px solid rgba(7, 193, 96, 0.35);
  outline-offset: 1px;
}

.auth-submit {
  margin-top: 4px;
  height: 44px;
  border-radius: var(--radius-sm);
  border: none;
  background: linear-gradient(120deg, #15c86b, #08b75b);
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.auth-submit:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.auth-error {
  margin: 0;
  color: var(--danger);
  font-size: 13px;
}

.auth-footer {
  margin-top: 18px;
  color: var(--text-soft);
  font-size: 14px;
}

.auth-footer a {
  color: var(--primary-strong);
  font-weight: 600;
  text-decoration: none;
}
</style>
