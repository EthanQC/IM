import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { apiGetProfile, apiLogin, apiRegister, apiUpdateProfile } from "../services/api";
import { clearProfile, clearTokens, getAccessToken, getRefreshToken, loadProfile, saveProfile, setTokens } from "../services/token";
import { toErrorMessage } from "../services/response";
import type { UserProfile } from "../types/im";

export const useAuthStore = defineStore("auth", () => {
  const accessToken = ref(getAccessToken());
  const refreshToken = ref(getRefreshToken());
  const profile = ref<UserProfile | null>(loadProfile<UserProfile>());
  const initializing = ref(false);

  const isLoggedIn = computed(() => Boolean(accessToken.value));
  const userId = computed(() => Number(profile.value?.user?.id || 0));
  const userName = computed(() => profile.value?.user?.display_name || profile.value?.user?.username || "");

  function syncTokenState(): void {
    accessToken.value = getAccessToken();
    refreshToken.value = getRefreshToken();
  }

  function applyAuth(payload: {
    access_token: string;
    refresh_token: string;
    profile?: UserProfile;
  }): void {
    setTokens(payload.access_token, payload.refresh_token);
    syncTokenState();

    if (payload.profile) {
      profile.value = payload.profile;
      saveProfile(payload.profile);
    }
  }

  async function login(input: { username: string; password: string }): Promise<void> {
    const payload = await apiLogin(input);
    applyAuth(payload);

    if (!payload.profile) {
      await refreshProfile();
    }
  }

  async function register(input: { username: string; password: string; display_name?: string }): Promise<void> {
    const payload = await apiRegister(input);
    if (payload.access_token && payload.refresh_token) {
      applyAuth(payload);

      if (!payload.profile) {
        await refreshProfile();
      }
      return;
    }

    // 兼容后端 register 成功但未返回 token 的情况：自动走一次 login。
    await login({
      username: input.username,
      password: input.password,
    });
  }

  async function refreshProfile(): Promise<UserProfile | null> {
    if (!accessToken.value) {
      return null;
    }

    try {
      const nextProfile = await apiGetProfile();
      profile.value = nextProfile;
      saveProfile(nextProfile);
      return nextProfile;
    } catch (error) {
      throw new Error(toErrorMessage(error));
    }
  }

  async function updateProfile(input: { display_name?: string; avatar_url?: string }): Promise<UserProfile> {
    const nextProfile = await apiUpdateProfile(input);
    profile.value = nextProfile;
    saveProfile(nextProfile);
    return nextProfile;
  }

  function logout(): void {
    clearTokens();
    clearProfile();
    accessToken.value = "";
    refreshToken.value = "";
    profile.value = null;
  }

  async function bootstrap(): Promise<void> {
    if (initializing.value) {
      return;
    }

    if (!accessToken.value) {
      return;
    }

    initializing.value = true;
    try {
      if (!profile.value) {
        await refreshProfile();
      }
    } finally {
      syncTokenState();
      initializing.value = false;
    }
  }

  return {
    accessToken,
    refreshToken,
    profile,
    initializing,
    isLoggedIn,
    userId,
    userName,
    login,
    register,
    refreshProfile,
    updateProfile,
    logout,
    bootstrap,
  };
});
