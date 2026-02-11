import type { MediaRef, NormalizedMessage } from "../types/im";

function parseUnix(value: any): number {
  if (!value) {
    return Math.floor(Date.now() / 1000);
  }

  if (typeof value === "number") {
    return value > 2_000_000_000 ? Math.floor(value / 1000) : Math.floor(value);
  }

  if (typeof value === "string") {
    const parsed = Number(value);
    if (!Number.isNaN(parsed)) {
      return parsed > 2_000_000_000 ? Math.floor(parsed / 1000) : Math.floor(parsed);
    }
    return Math.floor(Date.parse(value) / 1000);
  }

  if (typeof value === "object" && typeof value.seconds === "number") {
    return Math.floor(value.seconds);
  }

  return Math.floor(Date.now() / 1000);
}

function pickBodyCarrier(body: any): any {
  if (!body || typeof body !== "object") {
    return null;
  }

  return body.Body || body.body || body;
}

function pickText(body: any): string | undefined {
  return (
    body?.text?.text ||
    body?.Text?.text ||
    body?.text?.Text ||
    body?.Text?.Text ||
    undefined
  );
}

function pickCallHint(body: any): string | undefined {
  return body?.call?.convo_hint || body?.Call?.convo_hint || body?.call?.convoHint || body?.Call?.convoHint;
}

function pickMedia(body: any): { kind: "image" | "file" | "audio" | "video"; media: MediaRef } | null {
  const pairs: Array<["image" | "file" | "audio" | "video", any]> = [
    ["image", body?.image || body?.Image],
    ["file", body?.file || body?.File],
    ["audio", body?.audio || body?.Audio],
    ["video", body?.video || body?.Video],
  ];

  for (const [kind, value] of pairs) {
    if (value && typeof value === "object") {
      return {
        kind,
        media: {
          object_key: value.object_key || value.objectKey || "",
          filename: value.filename || "",
          content_type: value.content_type || value.contentType || "",
          size_bytes: Number(value.size_bytes || value.sizeBytes || 0),
          duration_sec: value.duration_sec || value.durationSec,
          thumbnail_key: value.thumbnail_key || value.thumbnailKey,
        },
      };
    }
  }

  return null;
}

function inferKind(contentType: number): NormalizedMessage["kind"] {
  switch (contentType) {
    case 1:
      return "text";
    case 2:
      return "image";
    case 3:
      return "file";
    case 4:
      return "audio";
    case 5:
      return "video";
    case 6:
    case 7:
    case 8:
    case 9:
      return "call";
    default:
      return "unknown";
  }
}

function parseContent(rawBody: any, contentType: number, rawContent?: any): Pick<NormalizedMessage, "kind" | "text" | "media" | "callHint" | "rawBody"> {
  let bodyValue = pickBodyCarrier(rawBody);

  if (!bodyValue && rawContent != null) {
    if (typeof rawContent === "string") {
      try {
        bodyValue = JSON.parse(rawContent);
      } catch {
        if (contentType === 1) {
          return {
            kind: "text",
            text: rawContent,
            rawBody: rawContent,
          };
        }

        return {
          kind: inferKind(contentType),
          rawBody: rawContent,
        };
      }
    } else {
      bodyValue = rawContent;
    }
  }

  const text = pickText(bodyValue);
  if (text) {
    return {
      kind: "text",
      text,
      rawBody: bodyValue,
    };
  }

  const mediaInfo = pickMedia(bodyValue);
  if (mediaInfo) {
    return {
      kind: mediaInfo.kind,
      media: mediaInfo.media,
      rawBody: bodyValue,
    };
  }

  const callHint = pickCallHint(bodyValue);
  if (callHint) {
    return {
      kind: "call",
      callHint,
      rawBody: bodyValue,
    };
  }

  return {
    kind: inferKind(contentType),
    rawBody: bodyValue,
  };
}

export function normalizeApiMessage(raw: any): NormalizedMessage {
  const contentType = Number(raw?.content_type || 0);
  const content = parseContent(raw?.body, contentType);

  return {
    id: Number(raw?.id || 0),
    conversationId: Number(raw?.conversation_id || 0),
    senderId: Number(raw?.sender_id || 0),
    seq: Number(raw?.seq || 0),
    contentType,
    createdAtUnix: parseUnix(raw?.create_time),
    revoked: false,
    ...content,
  };
}

export function normalizeWsNewMessage(raw: any): NormalizedMessage {
  const contentType = Number(raw?.content_type || 0);
  const content = parseContent(undefined, contentType, raw?.content);

  return {
    id: Number(raw?.message_id || 0),
    conversationId: Number(raw?.conversation_id || 0),
    senderId: Number(raw?.sender_id || 0),
    seq: Number(raw?.seq || 0),
    contentType,
    createdAtUnix: parseUnix(raw?.created_at),
    revoked: false,
    ...content,
  };
}

export function upsertMessage(messages: NormalizedMessage[], nextMessage: NormalizedMessage): NormalizedMessage[] {
  const existingIndex = messages.findIndex((item) => item.id === nextMessage.id || (item.seq > 0 && item.seq === nextMessage.seq));
  if (existingIndex === -1) {
    return [...messages, nextMessage].sort((a, b) => a.seq - b.seq || a.id - b.id);
  }

  const copy = [...messages];
  copy[existingIndex] = {
    ...copy[existingIndex],
    ...nextMessage,
  };
  return copy.sort((a, b) => a.seq - b.seq || a.id - b.id);
}

export function formatMessageTime(unixSeconds: number): string {
  const date = new Date(unixSeconds * 1000);
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}
