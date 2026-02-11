import { nanoid } from "nanoid";
import type { WsEnvelope } from "../types/im";

interface ClientOptions {
  userId: number;
  deviceId: string;
  onOpen?: () => void;
  onClose?: (reason: string) => void;
  onError?: (message: string) => void;
  onMessage?: (message: WsEnvelope) => void;
}

interface PendingRequest {
  resolve: (value: any) => void;
  reject: (reason?: unknown) => void;
  timer: number;
}

function normalizeWsBaseURL(raw: string): string {
  const source = raw || "/ws";

  if (source.startsWith("ws://") || source.startsWith("wss://")) {
    return source;
  }

  if (source.startsWith("http://") || source.startsWith("https://")) {
    return source.replace(/^http/, "ws");
  }

  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const path = source.startsWith("/") ? source : `/${source}`;
  return `${protocol}//${window.location.host}${path}`;
}

function buildSocketURL(baseURL: string, userId: number, deviceId: string): string {
  const url = new URL(normalizeWsBaseURL(baseURL));
  url.searchParams.set("user_id", String(userId));
  url.searchParams.set("device_id", deviceId);
  url.searchParams.set("platform", "web");
  return url.toString();
}

export class IMWebSocketClient {
  private readonly options: ClientOptions;
  private ws: WebSocket | null = null;
  private manualClose = false;
  private reconnectAttempts = 0;
  private reconnectTimer: number | null = null;
  private readonly pending = new Map<string, PendingRequest>();

  constructor(options: ClientOptions) {
    this.options = options;
  }

  get isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  connect(): void {
    this.manualClose = false;

    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return;
    }

    const wsBaseURL = import.meta.env.VITE_WS_BASE_URL || "/ws";
    const socketURL = buildSocketURL(wsBaseURL, this.options.userId, this.options.deviceId);

    this.ws = new WebSocket(socketURL);

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
      this.options.onOpen?.();
    };

    this.ws.onclose = (event) => {
      this.options.onClose?.(event.reason || `code:${event.code}`);
      this.ws = null;

      if (!this.manualClose) {
        this.scheduleReconnect();
      }
    };

    this.ws.onerror = () => {
      this.options.onError?.("WebSocket 连接异常");
    };

    this.ws.onmessage = (event) => {
      try {
        const envelope = JSON.parse(event.data) as WsEnvelope;

        if (envelope.type === "signal_resp" && envelope.id && this.pending.has(envelope.id)) {
          const pending = this.pending.get(envelope.id)!;
          window.clearTimeout(pending.timer);
          this.pending.delete(envelope.id);
          pending.resolve(envelope.data);
          return;
        }

        if (envelope.type === "error" && envelope.id && this.pending.has(envelope.id)) {
          const pending = this.pending.get(envelope.id)!;
          window.clearTimeout(pending.timer);
          this.pending.delete(envelope.id);
          const message = envelope.data?.error || "信令请求失败";
          pending.reject(new Error(message));
          return;
        }

        this.options.onMessage?.(envelope);
      } catch {
        this.options.onError?.("WebSocket 消息解析失败");
      }
    };
  }

  disconnect(): void {
    this.manualClose = true;

    if (this.reconnectTimer) {
      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    this.pending.forEach((request) => {
      window.clearTimeout(request.timer);
      request.reject(new Error("WebSocket disconnected"));
    });
    this.pending.clear();

    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  send(type: string, data?: any, id?: string): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error("WebSocket 未连接");
    }

    this.ws.send(
      JSON.stringify({
        type,
        id,
        data,
        ts: Date.now(),
      }),
    );
  }

  sendAck(input: { conversation_id: number; message_id: number; seq: number }): void {
    this.send("ack", {
      conversation_id: input.conversation_id,
      message_id: input.message_id,
      seq: input.seq,
    });
  }

  async sendSignaling(action: string, payload: any, timeoutMs = 15000): Promise<any> {
    const id = nanoid();

    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error("WebSocket 未连接");
    }

    const promise = new Promise<any>((resolve, reject) => {
      const timer = window.setTimeout(() => {
        this.pending.delete(id);
        reject(new Error("信令请求超时"));
      }, timeoutMs);

      this.pending.set(id, {
        resolve,
        reject,
        timer,
      });
    });

    this.send(
      "signaling",
      {
        action,
        payload,
      },
      id,
    );

    return promise;
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) {
      window.clearTimeout(this.reconnectTimer);
    }

    const delay = Math.min(8_000, 500 * 2 ** this.reconnectAttempts);
    this.reconnectAttempts += 1;

    this.reconnectTimer = window.setTimeout(() => {
      this.connect();
    }, delay);
  }
}
