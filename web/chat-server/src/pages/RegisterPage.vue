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
  displayName: "",
  password: "",
  confirmPassword: "",
});

async function submit(): Promise<void> {
  const username = form.username.trim();
  if (!username || !form.password) {
    errorMessage.value = "用户名和密码为必填";
    return;
  }

  if (form.password.length < 6) {
    errorMessage.value = "密码长度至少 6 位";
    return;
  }

  if (form.password !== form.confirmPassword) {
    errorMessage.value = "两次密码输入不一致";
    return;
  }

  loading.value = true;
  errorMessage.value = "";

  try {
    await auth.register({
      username,
      password: form.password,
      display_name: form.displayName.trim() || undefined,
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
    <section class="auth-card register-card">
      <header class="auth-header">
        <p class="auth-kicker">IM</p>
        <h1>创建账号</h1>
        <p class="auth-subtitle">注册后可直接进入会话、联系人与实时通话</p>
      </header>

      <form class="auth-form" @submit.prevent="submit">
        <label>
          <span>用户名</span>
          <input v-model="form.username" autocomplete="username" placeholder="至少 3 个字符" />
        </label>

        <label>
          <span>显示名称（可选）</span>
          <input v-model="form.displayName" autocomplete="nickname" placeholder="用于聊天展示" />
        </label>

        <label>
          <span>密码</span>
          <input v-model="form.password" type="password" autocomplete="new-password" placeholder="至少 6 位" />
        </label>

        <label>
          <span>确认密码</span>
          <input
            v-model="form.confirmPassword"
            type="password"
            autocomplete="new-password"
            placeholder="再次输入密码"
          />
        </label>

        <p v-if="errorMessage" class="auth-error">{{ errorMessage }}</p>

        <button class="auth-submit" :disabled="loading" type="submit">
          {{ loading ? "注册中..." : "注册并登录" }}
        </button>
      </form>

      <footer class="auth-footer">
        已有账号？
        <RouterLink to="/login">返回登录</RouterLink>
      </footer>
    </section>
  </main>
</template>

<style scoped>
.register-card {
  width: min(520px, 96vw);
}

.auth-header {
  margin-bottom: 20px;
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
  outline: 2px solid #ffcade;
  outline-offset: 1px;
}

.auth-submit {
  margin-top: 4px;
  height: 44px;
  border-radius: var(--radius-sm);
  border: none;
  background: linear-gradient(120deg, #f7a8c6, #f18ab2);
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
