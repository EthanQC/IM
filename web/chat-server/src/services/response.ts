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

function normalizeServerErrorMessage(message: string): string {
  let raw = (message || "").trim();

  if (raw.includes("rpc error: code =")) {
    const descMatch = raw.match(/desc\s*=\s*(.+)$/i);
    if (descMatch?.[1]) {
      raw = descMatch[1].trim();
    } else {
      return "服务暂时不可用，请稍后重试";
    }
  }

  if (raw.includes("context deadline exceeded")) {
    return "服务响应超时，请稍后重试";
  }
  if (raw.includes("Failed to fetch")) {
    return "网络连接失败，请检查服务和网络后重试";
  }
  if (raw.includes("Requested device not found")) {
    return "未检测到可用的麦克风或摄像头，请检查设备与浏览器权限";
  }
  if (raw.includes("NotAllowedError")) {
    return "浏览器未授予麦克风/摄像头权限，请在地址栏中允许后重试";
  }
  if (raw.includes("upload proxy failed")) {
    return "文件上传失败，请检查本地上传代理或网络";
  }

  if (raw.includes("cannot add yourself as contact")) {
    return "不能添加自己为联系人";
  }
  if (raw.includes("already contact")) {
    return "你们已经是联系人了";
  }
  if (raw.includes("contact apply already exists")) {
    return "好友申请已发送，请等待对方处理";
  }
  if (raw.includes("contact apply not found")) {
    return "未找到待处理的好友申请";
  }
  if (raw.includes("target user not found")) {
    return "目标用户不存在";
  }
  if (raw.includes("contact is blocked")) {
    return "由于黑名单限制，无法添加该联系人";
  }
  if (raw.includes("invalid token")) {
    return "登录状态已失效，请重新登录";
  }
  if (raw.includes("register succeeded but login failed")) {
    return "注册成功，但自动登录失败，请手动登录";
  }

  return raw;
}

export function toErrorMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const body = error.response?.data;
    if (typeof body === "string") {
      return normalizeServerErrorMessage(body);
    }
    if (body?.error) {
      return normalizeServerErrorMessage(String(body.error));
    }
    if (body?.message) {
      return normalizeServerErrorMessage(String(body.message));
    }
    if (error.message) {
      return normalizeServerErrorMessage(error.message);
    }
  }

  if (error instanceof Error) {
    return normalizeServerErrorMessage(error.message);
  }

  return "请求失败";
}
