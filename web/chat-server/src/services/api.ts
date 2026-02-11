import { http } from "./http";
import { ensureAuthPayload, unwrapData } from "./response";
import type {
  AuthPayload,
  ConversationBrief,
  NormalizedMessage,
  PresenceItem,
  UploadTicket,
  UserBrief,
  UserProfile,
} from "../types/im";
import { normalizeApiMessage } from "../utils/message";

export async function apiRegister(input: {
  username: string;
  password: string;
  display_name?: string;
}): Promise<AuthPayload> {
  const response = await http.post("/auth/register", input);
  return ensureAuthPayload(response.data);
}

export async function apiLogin(input: { username: string; password: string }): Promise<AuthPayload> {
  const response = await http.post("/auth/login", input);
  return ensureAuthPayload(response.data);
}

export async function apiGetProfile(): Promise<UserProfile> {
  const response = await http.get("/users/me");
  return unwrapData<UserProfile>(response.data);
}

export async function apiUpdateProfile(input: { display_name?: string; avatar_url?: string }): Promise<UserProfile> {
  const response = await http.put("/users/me", input);
  return unwrapData<UserProfile>(response.data);
}

export async function apiListContacts(): Promise<UserBrief[]> {
  const response = await http.get("/contacts");
  return unwrapData<UserBrief[]>(response.data) || [];
}

export async function apiApplyContact(input: { target_user_id: number; remark?: string }): Promise<void> {
  await http.post("/contacts/apply", input);
}

export async function apiHandleContact(input: { target_user_id: number; accept: boolean }): Promise<void> {
  await http.post("/contacts/handle", input);
}

export async function apiDeleteContact(targetUserId: number): Promise<void> {
  await http.delete(`/contacts/${targetUserId}`);
}

export async function apiListConversations(): Promise<ConversationBrief[]> {
  const response = await http.get("/conversations");
  return unwrapData<ConversationBrief[]>(response.data) || [];
}

export async function apiCreateConversation(input: {
  type: 1 | 2;
  title?: string;
  member_ids: number[];
}): Promise<ConversationBrief> {
  const response = await http.post("/conversations", input);
  return unwrapData<ConversationBrief>(response.data);
}

export async function apiGetConversation(conversationId: number): Promise<ConversationBrief> {
  const response = await http.get(`/conversations/${conversationId}`);
  return unwrapData<ConversationBrief>(response.data);
}

export async function apiUpdateConversation(conversationId: number, input: { title: string }): Promise<ConversationBrief> {
  const response = await http.put(`/conversations/${conversationId}`, input);
  return unwrapData<ConversationBrief>(response.data);
}

export async function apiSendTextMessage(input: {
  conversation_id: number;
  client_msg_id: string;
  content_type?: number;
  text: string;
}): Promise<NormalizedMessage> {
  const response = await http.post("/messages", {
    conversation_id: input.conversation_id,
    client_msg_id: input.client_msg_id,
    content_type: input.content_type || 1,
    text: input.text,
  });

  const raw = unwrapData<any>(response.data);
  return normalizeApiMessage(raw);
}

export async function apiGetHistory(conversationId: number, afterSeq = 0, limit = 100): Promise<NormalizedMessage[]> {
  const response = await http.get("/messages/history", {
    params: {
      conversation_id: conversationId,
      after_seq: afterSeq,
      limit,
    },
  });

  const rawList = unwrapData<any[]>(response.data) || [];
  return rawList.map(normalizeApiMessage);
}

export async function apiMarkRead(conversationId: number, readSeq: number): Promise<void> {
  if (readSeq <= 0) {
    return;
  }
  await http.post("/messages/read", {
    conversation_id: conversationId,
    read_seq: readSeq,
  });
}

export async function apiRevokeMessage(messageId: number): Promise<void> {
  await http.post(`/messages/${messageId}/revoke`);
}

export async function apiGetPresence(userIds: number[]): Promise<PresenceItem[]> {
  if (!userIds.length) {
    return [];
  }

  const query = userIds.map((id) => `user_ids=${encodeURIComponent(String(id))}`).join("&");
  const response = await http.get(`/presence?${query}`);
  return unwrapData<PresenceItem[]>(response.data) || [];
}

export async function apiCreateUpload(input: {
  filename: string;
  content_type: string;
  size_bytes: number;
  kind: "image" | "video" | "audio" | "file";
}): Promise<UploadTicket> {
  const response = await http.post("/files/upload", input);
  return unwrapData<UploadTicket>(response.data);
}

export async function apiPutFile(uploadURL: string, file: File): Promise<void> {
  const result = await fetch(uploadURL, {
    method: "PUT",
    headers: {
      "Content-Type": file.type || "application/octet-stream",
    },
    body: file,
  });

  if (!result.ok) {
    throw new Error(`文件上传失败: ${result.status}`);
  }
}

export async function apiCompleteUpload(input: {
  conversation_id: number;
  client_msg_id: string;
  object_key: string;
}): Promise<NormalizedMessage> {
  const response = await http.post("/files/complete", input);
  const raw = unwrapData<any>(response.data);
  return normalizeApiMessage(raw);
}
