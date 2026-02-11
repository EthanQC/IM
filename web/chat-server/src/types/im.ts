export interface UserBrief {
  id: number;
  username: string;
  display_name: string;
  avatar_url: string;
}

export interface UserProfile {
  user: UserBrief;
  status: string;
}

export interface AuthPayload {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  profile?: UserProfile;
}

export interface ConversationBrief {
  id: number;
  type: number;
  title: string;
}

export interface PresenceItem {
  user_id: number;
  online: boolean;
  node_id: string;
  last_seen_unix: number;
}

export interface MediaRef {
  object_key: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  duration_sec?: number;
  thumbnail_key?: string;
}

export type MessageKind = "text" | "image" | "file" | "audio" | "video" | "call" | "unknown";

export interface NormalizedMessage {
  id: number;
  conversationId: number;
  senderId: number;
  seq: number;
  contentType: number;
  kind: MessageKind;
  text?: string;
  media?: MediaRef;
  callHint?: string;
  createdAtUnix: number;
  revoked: boolean;
  rawBody?: unknown;
}

export interface UploadTicket {
  object_key: string;
  upload_url: string;
  callback_url?: string;
}

export interface WsEnvelope {
  type: string;
  id?: string;
  data?: any;
  ts?: number;
}

export interface SignalingPayload {
  action: string;
  call_id?: string;
  from_user?: number;
  from_device?: string;
  payload?: any;
  timestamp?: number;
}
