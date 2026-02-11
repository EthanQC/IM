const ACCESS_TOKEN_KEY = "im.access_token";
const REFRESH_TOKEN_KEY = "im.refresh_token";
const PROFILE_KEY = "im.profile";

function readSession(key: string): string {
  try {
    return sessionStorage.getItem(key) || "";
  } catch {
    return "";
  }
}

function writeSession(key: string, value: string): void {
  try {
    sessionStorage.setItem(key, value);
  } catch {
    // ignore storage failures
  }
}

function removeSession(key: string): void {
  try {
    sessionStorage.removeItem(key);
  } catch {
    // ignore storage failures
  }
}

function readWithLegacyFallback(key: string): string {
  const sessionValue = readSession(key);
  if (sessionValue) {
    return sessionValue;
  }

  // 兼容旧版本 localStorage，首次读取后迁移到 sessionStorage，避免多窗口互相覆盖登录态。
  const legacyValue = localStorage.getItem(key) || "";
  if (legacyValue) {
    writeSession(key, legacyValue);
    localStorage.removeItem(key);
  }
  return legacyValue;
}

export function getAccessToken(): string {
  return readWithLegacyFallback(ACCESS_TOKEN_KEY);
}

export function getRefreshToken(): string {
  return readWithLegacyFallback(REFRESH_TOKEN_KEY);
}

export function setTokens(accessToken: string, refreshToken: string): void {
  writeSession(ACCESS_TOKEN_KEY, accessToken);
  writeSession(REFRESH_TOKEN_KEY, refreshToken);
}

export function clearTokens(): void {
  removeSession(ACCESS_TOKEN_KEY);
  removeSession(REFRESH_TOKEN_KEY);
  // 清除旧版本遗留，避免迁移后再次被读取
  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
}

export function saveProfile(profile: unknown): void {
  writeSession(PROFILE_KEY, JSON.stringify(profile));
}

export function loadProfile<T>(): T | null {
  const raw = readWithLegacyFallback(PROFILE_KEY);
  if (!raw) {
    return null;
  }

  try {
    return JSON.parse(raw) as T;
  } catch {
    return null;
  }
}

export function clearProfile(): void {
  removeSession(PROFILE_KEY);
  localStorage.removeItem(PROFILE_KEY);
}
