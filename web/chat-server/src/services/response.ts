import axios from "axios";

export function unwrapData<T>(payload: any): T {
  if (payload && typeof payload === "object" && "code" in payload && "data" in payload) {
    return payload.data as T;
  }
  return payload as T;
}

export function ensureAuthPayload(payload: any): {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  profile?: any;
} {
  const unwrapped = unwrapData<any>(payload);
  if (!unwrapped?.access_token || !unwrapped?.refresh_token) {
    throw new Error("invalid auth payload");
  }
  return unwrapped;
}

export function toErrorMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const body = error.response?.data;
    if (typeof body === "string") {
      return body;
    }
    if (body?.error) {
      return String(body.error);
    }
    if (body?.message) {
      return String(body.message);
    }
    if (error.message) {
      return error.message;
    }
  }

  if (error instanceof Error) {
    return error.message;
  }

  return "请求失败";
}
