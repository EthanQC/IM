import axios, { type InternalAxiosRequestConfig } from "axios";
import { clearProfile, clearTokens, getAccessToken, getRefreshToken, setTokens } from "./token";
import { ensureAuthPayload } from "./response";

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "/api";

interface RetryConfig extends InternalAxiosRequestConfig {
  _retry?: boolean;
}

const rawHttp = axios.create({
  baseURL: API_BASE_URL,
  timeout: 20000,
});

export const http = axios.create({
  baseURL: API_BASE_URL,
  timeout: 20000,
});

let refreshPromise: Promise<string> | null = null;

async function refreshAccessToken(): Promise<string> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) {
    throw new Error("missing refresh token");
  }

  const response = await rawHttp.post("/auth/refresh", {
    refresh_token: refreshToken,
  });

  const payload = ensureAuthPayload(response.data);
  setTokens(payload.access_token, payload.refresh_token);
  return payload.access_token;
}

http.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

http.interceptors.response.use(
  (response) => response,
  async (error) => {
    const status = error?.response?.status;
    const originalConfig = error?.config as RetryConfig | undefined;

    if (status !== 401 || !originalConfig || originalConfig._retry) {
      return Promise.reject(error);
    }

    originalConfig._retry = true;

    try {
      if (!refreshPromise) {
        refreshPromise = refreshAccessToken().finally(() => {
          refreshPromise = null;
        });
      }

      const nextAccessToken = await refreshPromise;
      originalConfig.headers.Authorization = `Bearer ${nextAccessToken}`;
      return http(originalConfig);
    } catch (refreshError) {
      clearTokens();
      clearProfile();
      if (window.location.pathname !== "/login") {
        window.location.href = "/login";
      }
      return Promise.reject(refreshError);
    }
  },
);

export { API_BASE_URL };
