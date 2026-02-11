import { computed, reactive, ref, shallowRef } from "vue";
import { defineStore } from "pinia";
import { nanoid } from "nanoid";
import {
  apiApplyContact,
  apiCompleteUpload,
  apiCreateConversation,
  apiCreateUpload,
  apiDeleteContact,
  apiGetConversation,
  apiGetHistory,
  apiGetPresence,
  apiHandleContact,
  apiListContacts,
  apiListConversations,
  apiMarkRead,
  apiPutFile,
  apiRevokeMessage,
  apiSendTextMessage,
  apiUpdateConversation,
} from "../services/api";
import { IMWebSocketClient } from "../services/ws";
import { toErrorMessage } from "../services/response";
import { formatMessageTime, normalizeWsNewMessage, upsertMessage } from "../utils/message";
import type {
  ConversationBrief,
  NormalizedMessage,
  PresenceItem,
  SignalingPayload,
  UserBrief,
  WsEnvelope,
} from "../types/im";
import { useAuthStore } from "./auth";

type UploadKind = "image" | "video" | "audio" | "file";
type CallPhase = "idle" | "incoming" | "outgoing" | "connecting" | "connected";
type CallMediaType = "audio" | "video";

interface IncomingCall {
  callId: string;
  fromUserId: number;
  conversationId: number;
  callType: CallMediaType;
  stunServers: string[];
  turnServers: Array<{ urls: string[]; username?: string; credential?: string }>;
}

interface ActiveCall {
  callId: string;
  peerUserId: number;
  conversationId: number;
  callType: CallMediaType;
  stunServers: string[];
  turnServers: Array<{ urls: string[]; username?: string; credential?: string }>;
}

function inferUploadKind(file: File): UploadKind {
  if (file.type.startsWith("image/")) {
    return "image";
  }
  if (file.type.startsWith("video/")) {
    return "video";
  }
  if (file.type.startsWith("audio/")) {
    return "audio";
  }
  return "file";
}

function makeDeviceID(): string {
  const installKey = "im.installation_id";
  const runtimeKey = "__im_runtime_device_id";
  const runtime = (window as unknown as Record<string, string | undefined>)[runtimeKey];

  if (runtime) {
    return runtime;
  }

  let installationID = localStorage.getItem(installKey);
  if (!installationID) {
    installationID = `app-${nanoid(8)}`;
    localStorage.setItem(installKey, installationID);
  }

  // 每个页面实例独立 device_id，避免同账号多窗口（含复制标签页）互相顶掉连接。
  const next = `web-${installationID}-${nanoid(6)}`;
  (window as unknown as Record<string, string | undefined>)[runtimeKey] = next;
  return next;
}

function toFileURL(objectKey: string): string {
  const publicBase = import.meta.env.VITE_MINIO_PUBLIC_BASE_URL || "";
  if (!publicBase) {
    return objectKey;
  }
  const normalizedBase = publicBase.endsWith("/") ? publicBase.slice(0, -1) : publicBase;
  const normalizedKey = objectKey.startsWith("/") ? objectKey.slice(1) : objectKey;
  return `${normalizedBase}/${normalizedKey}`;
}

export const useIMStore = defineStore("im", () => {
  const auth = useAuthStore();

  const initializing = ref(false);
  const loading = ref(false);
  const wsConnected = ref(false);
  const wsError = ref("");
  const errorMessage = ref("");

  const conversations = ref<ConversationBrief[]>([]);
  const contacts = ref<UserBrief[]>([]);
  const presenceMap = ref<Record<number, PresenceItem>>({});
  const unreadMap = ref<Record<number, number>>({});
  const readMap = ref<Record<number, { userId: number; seq: number; readAt: number }>>({});
  const messageMap = ref<Record<number, NormalizedMessage[]>>({});

  const activeConversationId = ref<number | null>(null);
  const sending = ref(false);
  const uploading = ref(false);

  const callPhase = ref<CallPhase>("idle");
  const incomingCall = ref<IncomingCall | null>(null);
  const activeCall = ref<ActiveCall | null>(null);
  const callError = ref("");
  const localStream = shallowRef<MediaStream | null>(null);
  const remoteStream = shallowRef<MediaStream | null>(null);

  const wsClient = shallowRef<IMWebSocketClient | null>(null);
  const peerConnection = shallowRef<RTCPeerConnection | null>(null);
  const readTimerMap = new Map<number, number>();

  const activeConversation = computed(() => {
    if (!activeConversationId.value) {
      return null;
    }
    return conversations.value.find((item) => item.id === activeConversationId.value) || null;
  });

  const activeMessages = computed(() => {
    if (!activeConversationId.value) {
      return [] as NormalizedMessage[];
    }
    return messageMap.value[activeConversationId.value] || [];
  });

  const isCalling = computed(() => callPhase.value !== "idle");

  function resetError(): void {
    errorMessage.value = "";
  }

  function setError(error: unknown): void {
    errorMessage.value = toErrorMessage(error);
  }

  function setMessages(conversationId: number, nextMessages: NormalizedMessage[]): void {
    messageMap.value = {
      ...messageMap.value,
      [conversationId]: nextMessages,
    };
  }

  function upsertConversation(nextConversation: ConversationBrief): void {
    const existing = conversations.value.find((item) => item.id === nextConversation.id);

    if (!existing) {
      conversations.value = [nextConversation, ...conversations.value];
      return;
    }

    conversations.value = [
      { ...existing, ...nextConversation },
      ...conversations.value.filter((item) => item.id !== nextConversation.id),
    ];
  }

  function ensureConversationPlaceholder(conversationId: number): void {
    if (conversations.value.some((item) => item.id === conversationId)) {
      return;
    }

    conversations.value = [
      {
        id: conversationId,
        type: 1,
        title: `会话 #${conversationId}`,
      },
      ...conversations.value,
    ];
  }

  async function initialize(): Promise<void> {
    if (initializing.value) {
      return;
    }

    initializing.value = true;
    resetError();

    try {
      await auth.bootstrap();
      await Promise.all([loadConversations(), loadContacts()]);
      await refreshPresence();

      if (!activeConversationId.value && conversations.value.length > 0) {
        await openConversation(conversations.value[0].id);
      }

      connectRealtime();
    } catch (error) {
      setError(error);
    } finally {
      initializing.value = false;
    }
  }

  function resetState(): void {
    conversations.value = [];
    contacts.value = [];
    presenceMap.value = {};
    unreadMap.value = {};
    readMap.value = {};
    messageMap.value = {};
    activeConversationId.value = null;
    wsError.value = "";
    errorMessage.value = "";
    loading.value = false;
    sending.value = false;
    uploading.value = false;
    disconnectRealtime();
    resetCallState();
  }

  async function loadConversations(): Promise<void> {
    loading.value = true;
    resetError();

    try {
      const items = await apiListConversations();
      conversations.value = items;

      if (activeConversationId.value && !items.some((item) => item.id === activeConversationId.value)) {
        activeConversationId.value = items[0]?.id || null;
      }
    } catch (error) {
      setError(error);
    } finally {
      loading.value = false;
    }
  }

  async function loadContacts(): Promise<void> {
    resetError();

    try {
      contacts.value = await apiListContacts();
    } catch (error) {
      setError(error);
    }
  }

  async function refreshPresence(): Promise<void> {
    try {
      const ids = contacts.value.map((item) => item.id);
      if (!ids.length) {
        presenceMap.value = {};
        return;
      }

      const records = await apiGetPresence(ids);
      const mapped: Record<number, PresenceItem> = {};
      records.forEach((item) => {
        mapped[item.user_id] = item;
      });
      presenceMap.value = mapped;
    } catch (error) {
      setError(error);
    }
  }

  async function openConversation(conversationId: number): Promise<void> {
    activeConversationId.value = conversationId;
    unreadMap.value = {
      ...unreadMap.value,
      [conversationId]: 0,
    };

    const existingMessages = messageMap.value[conversationId];
    if (!existingMessages || !existingMessages.length) {
      await loadHistory(conversationId);
    } else {
      scheduleMarkRead(conversationId, 250);
    }
  }

  async function loadHistory(conversationId: number): Promise<void> {
    resetError();

    try {
      const history = await apiGetHistory(conversationId, 0, 100);
      const sorted = [...history].sort((a, b) => a.seq - b.seq || a.id - b.id);
      setMessages(conversationId, sorted);
      await markConversationRead(conversationId);
    } catch (error) {
      setError(error);
    }
  }

  async function createConversation(input: {
    type: 1 | 2;
    title?: string;
    memberIDs: number[];
  }): Promise<ConversationBrief> {
    resetError();

    try {
      if (!input.memberIDs.length) {
        throw new Error("至少填写一个成员ID");
      }

      const conversation = await apiCreateConversation({
        type: input.type,
        title: input.title,
        member_ids: input.memberIDs,
      });

      upsertConversation(conversation);
      await openConversation(conversation.id);
      return conversation;
    } catch (error) {
      setError(error);
      throw error;
    }
  }

  async function refreshConversationDetail(conversationId: number): Promise<void> {
    try {
      const detail = await apiGetConversation(conversationId);
      upsertConversation(detail);
    } catch (error) {
      setError(error);
    }
  }

  async function renameConversation(conversationId: number, title: string): Promise<void> {
    resetError();

    try {
      const updated = await apiUpdateConversation(conversationId, { title });
      upsertConversation(updated);
    } catch (error) {
      setError(error);
      throw error;
    }
  }

  async function startSingleChatWithUser(targetUserId: number): Promise<void> {
    await createConversation({
      type: 1,
      memberIDs: [targetUserId],
    });
  }

  async function sendTextMessage(text: string): Promise<void> {
    if (!activeConversationId.value) {
      throw new Error("请先选择会话");
    }

    const content = text.trim();
    if (!content) {
      return;
    }

    sending.value = true;
    resetError();

    try {
      const sentMessage = await apiSendTextMessage({
        conversation_id: activeConversationId.value,
        client_msg_id: nanoid(),
        content_type: 1,
        text: content,
      });

      const list = messageMap.value[activeConversationId.value] || [];
      setMessages(activeConversationId.value, upsertMessage(list, sentMessage));
      scheduleMarkRead(activeConversationId.value, 120);
    } catch (error) {
      setError(error);
      throw error;
    } finally {
      sending.value = false;
    }
  }

  async function sendFileMessage(file: File): Promise<void> {
    if (!activeConversationId.value) {
      throw new Error("请先选择会话");
    }

    uploading.value = true;
    resetError();

    try {
      const kind = inferUploadKind(file);
      const upload = await apiCreateUpload({
        filename: file.name,
        content_type: file.type || "application/octet-stream",
        size_bytes: file.size,
        kind,
      });

      await apiPutFile(upload.upload_url, file);
      const message = await apiCompleteUpload({
        conversation_id: activeConversationId.value,
        client_msg_id: nanoid(),
        object_key: upload.object_key,
      });

      const list = messageMap.value[activeConversationId.value] || [];
      setMessages(activeConversationId.value, upsertMessage(list, message));
      scheduleMarkRead(activeConversationId.value, 120);
    } catch (error) {
      setError(error);
      throw error;
    } finally {
      uploading.value = false;
    }
  }

  async function revokeMessage(messageId: number): Promise<void> {
    resetError();

    try {
      await apiRevokeMessage(messageId);

      if (!activeConversationId.value) {
        return;
      }

      const list = messageMap.value[activeConversationId.value] || [];
      setMessages(
        activeConversationId.value,
        list.map((item) => (item.id === messageId ? { ...item, revoked: true } : item)),
      );
    } catch (error) {
      setError(error);
      throw error;
    }
  }

  async function markConversationRead(conversationId: number): Promise<void> {
    const list = messageMap.value[conversationId] || [];
    if (!list.length) {
      return;
    }

    const latestSeq = list[list.length - 1]?.seq || 0;
    if (latestSeq <= 0) {
      return;
    }

    try {
      await apiMarkRead(conversationId, latestSeq);
      unreadMap.value = {
        ...unreadMap.value,
        [conversationId]: 0,
      };
    } catch (error) {
      setError(error);
    }
  }

  function scheduleMarkRead(conversationId: number, delayMs = 500): void {
    if (readTimerMap.has(conversationId)) {
      window.clearTimeout(readTimerMap.get(conversationId)!);
      readTimerMap.delete(conversationId);
    }

    const timer = window.setTimeout(() => {
      readTimerMap.delete(conversationId);
      void markConversationRead(conversationId);
    }, delayMs);

    readTimerMap.set(conversationId, timer);
  }

  async function applyContact(targetUserId: number, remark: string): Promise<void> {
    resetError();
    try {
      await apiApplyContact({ target_user_id: targetUserId, remark });
    } catch (error) {
      setError(error);
      throw error;
    }
  }

  async function handleContact(targetUserId: number, accept: boolean): Promise<void> {
    resetError();
    try {
      await apiHandleContact({ target_user_id: targetUserId, accept });
      await loadContacts();
      await refreshPresence();
    } catch (error) {
      setError(error);
      throw error;
    }
  }

  async function deleteContact(targetUserId: number): Promise<void> {
    resetError();
    try {
      await apiDeleteContact(targetUserId);
      contacts.value = contacts.value.filter((item) => item.id !== targetUserId);
      const nextPresence = { ...presenceMap.value };
      delete nextPresence[targetUserId];
      presenceMap.value = nextPresence;
    } catch (error) {
      setError(error);
      throw error;
    }
  }

  function connectRealtime(): void {
    if (!auth.userId) {
      return;
    }

    disconnectRealtime();

    const client = new IMWebSocketClient({
      userId: auth.userId,
      deviceId: makeDeviceID(),
      onOpen: () => {
        wsConnected.value = true;
        wsError.value = "";
      },
      onClose: (reason) => {
        wsConnected.value = false;
        wsError.value = `连接已断开: ${reason}`;
      },
      onError: (message) => {
        wsError.value = message;
      },
      onMessage: (envelope) => {
        void handleRealtimeEnvelope(envelope);
      },
    });

    wsClient.value = client;
    client.connect();
  }

  function disconnectRealtime(): void {
    wsConnected.value = false;
    if (wsClient.value) {
      wsClient.value.disconnect();
      wsClient.value = null;
    }
  }

  async function handleRealtimeEnvelope(envelope: WsEnvelope): Promise<void> {
    if (!envelope || !envelope.type) {
      return;
    }

    switch (envelope.type) {
      case "notify":
        return;
      case "error":
        wsError.value = envelope.data?.error || "实时通道错误";
        return;
      case "new_message":
        await applyIncomingMessage(envelope.data);
        return;
      case "message_read":
        applyReadEvent(envelope.data);
        return;
      case "message_revoked":
        applyRevokeEvent(envelope.data);
        return;
      case "signaling":
        await handleSignalingEvent(envelope.data as SignalingPayload);
        return;
      default:
        return;
    }
  }

  async function applyIncomingMessage(raw: any): Promise<void> {
    if (!raw) {
      return;
    }

    const nextMessage = normalizeWsNewMessage(raw);
    const conversationId = nextMessage.conversationId;
    ensureConversationPlaceholder(conversationId);

    const currentList = messageMap.value[conversationId] || [];
    setMessages(conversationId, upsertMessage(currentList, nextMessage));

    try {
      wsClient.value?.sendAck({
        conversation_id: conversationId,
        message_id: nextMessage.id,
        seq: nextMessage.seq,
      });
    } catch {
      // ignore ack send error; reconnect will recover
    }

    if (activeConversationId.value === conversationId) {
      scheduleMarkRead(conversationId, 250);
    } else {
      unreadMap.value = {
        ...unreadMap.value,
        [conversationId]: (unreadMap.value[conversationId] || 0) + 1,
      };
    }

    const existing = conversations.value.find((item) => item.id === conversationId);
    if (existing) {
      upsertConversation(existing);
    }
  }

  function applyReadEvent(raw: any): void {
    if (!raw?.conversation_id) {
      return;
    }

    const conversationId = Number(raw.conversation_id);
    readMap.value = {
      ...readMap.value,
      [conversationId]: {
        userId: Number(raw.user_id || 0),
        seq: Number(raw.read_seq || 0),
        readAt: Number(raw.read_at || Math.floor(Date.now() / 1000)),
      },
    };
  }

  function applyRevokeEvent(raw: any): void {
    if (!raw?.conversation_id || !raw?.message_id) {
      return;
    }

    const conversationId = Number(raw.conversation_id);
    const messageId = Number(raw.message_id);
    const list = messageMap.value[conversationId] || [];

    if (!list.length) {
      return;
    }

    setMessages(
      conversationId,
      list.map((item) => (item.id === messageId ? { ...item, revoked: true } : item)),
    );
  }

  function getContactName(userId: number): string {
    const hit = contacts.value.find((item) => item.id === userId);
    if (!hit) {
      return `用户 ${userId}`;
    }
    return hit.display_name || hit.username || `用户 ${userId}`;
  }

  function formatReadTip(conversationId: number): string {
    const record = readMap.value[conversationId];
    if (!record || !record.seq) {
      return "";
    }
    return `${getContactName(record.userId)} 已读至 #${record.seq} (${formatMessageTime(record.readAt)})`;
  }

  function buildRTCIceConfig(call: ActiveCall): RTCConfiguration {
    const iceServers: RTCIceServer[] = [];

    call.stunServers.forEach((stun) => {
      iceServers.push({ urls: [stun] });
    });

    call.turnServers.forEach((turn) => {
      iceServers.push({
        urls: turn.urls,
        username: turn.username,
        credential: turn.credential,
      });
    });

    return {
      iceServers,
    };
  }

  async function ensureLocalStream(callType: CallMediaType): Promise<MediaStream | null> {
    if (localStream.value) {
      return localStream.value;
    }

    // 优先使用完整媒体能力；若本机无设备则降级为仅接收模式，避免通话流程直接失败。
    const candidates: MediaStreamConstraints[] =
      callType === "video"
        ? [
            { audio: true, video: true },
            { audio: true, video: false },
            { audio: false, video: true },
          ]
        : [{ audio: true, video: false }];

    let lastError: unknown = null;
    for (const constraints of candidates) {
      try {
        const stream = await navigator.mediaDevices.getUserMedia(constraints);
        localStream.value = stream;
        return stream;
      } catch (error) {
        lastError = error;
      }
    }

    if (lastError) {
      callError.value = toErrorMessage(lastError);
    }

    return null;
  }

  function stopLocalStream(): void {
    if (!localStream.value) {
      return;
    }

    localStream.value.getTracks().forEach((track) => {
      track.stop();
    });
    localStream.value = null;
  }

  function stopRemoteStream(): void {
    if (!remoteStream.value) {
      return;
    }

    remoteStream.value.getTracks().forEach((track) => {
      track.stop();
    });
    remoteStream.value = null;
  }

  async function ensurePeerConnection(call: ActiveCall): Promise<RTCPeerConnection> {
    if (peerConnection.value) {
      return peerConnection.value;
    }

    const connection = new RTCPeerConnection(buildRTCIceConfig(call));

    connection.ontrack = (event) => {
      const [stream] = event.streams;
      if (stream) {
        remoteStream.value = stream;
      }
    };

    connection.onicecandidate = (event) => {
      if (!event.candidate || !activeCall.value || !wsClient.value) {
        return;
      }

      void wsClient.value
        .sendSignaling("ice_candidate", {
          call_id: activeCall.value.callId,
          target_id: activeCall.value.peerUserId,
          candidate: event.candidate.candidate,
          sdp_mid: event.candidate.sdpMid,
          sdp_mline_index: event.candidate.sdpMLineIndex,
        })
        .catch(() => {
          // ignore
        });
    };

    connection.onconnectionstatechange = () => {
      if (connection.connectionState === "failed" || connection.connectionState === "disconnected") {
        callError.value = "通话连接断开";
      }
      if (connection.connectionState === "connected") {
        callPhase.value = "connected";
      }
    };

    const stream = await ensureLocalStream(call.callType);
    if (stream && stream.getTracks().length > 0) {
      stream.getTracks().forEach((track) => {
        connection.addTrack(track, stream);
      });
    } else {
      connection.addTransceiver("audio", { direction: "recvonly" });
      if (call.callType === "video") {
        connection.addTransceiver("video", { direction: "recvonly" });
      }
    }

    peerConnection.value = connection;
    return connection;
  }

  async function startOffer(call: ActiveCall): Promise<void> {
    if (!wsClient.value) {
      throw new Error("实时连接不可用");
    }

    const connection = await ensurePeerConnection(call);
    const offer = await connection.createOffer();
    await connection.setLocalDescription(offer);

    await wsClient.value.sendSignaling("offer", {
      call_id: call.callId,
      target_id: call.peerUserId,
      sdp: offer.sdp,
    });

    callPhase.value = "connecting";
  }

  async function initiateCall(targetUserId: number, callType: CallMediaType): Promise<void> {
    if (!activeConversationId.value) {
      throw new Error("请先选择会话");
    }

    if (!wsClient.value || !wsConnected.value) {
      throw new Error("实时连接未建立");
    }

    callError.value = "";

    try {
      const response = await wsClient.value.sendSignaling("call", {
        callee_id: targetUserId,
        conversation_id: activeConversationId.value,
        call_type: callType,
      });

      if (response?.status === "busy") {
        throw new Error("对方正在通话中");
      }

      const call: ActiveCall = {
        callId: response?.call_id,
        peerUserId: targetUserId,
        conversationId: activeConversationId.value,
        callType,
        stunServers: response?.stun_servers || [],
        turnServers: response?.turn_servers || [],
      };

      activeCall.value = call;
      callPhase.value = "outgoing";
    } catch (error) {
      callError.value = toErrorMessage(error);
      throw error;
    }
  }

  async function acceptIncomingCall(): Promise<void> {
    if (!incomingCall.value || !wsClient.value) {
      return;
    }

    callError.value = "";

    try {
      const incoming = incomingCall.value;
      await wsClient.value.sendSignaling("accept", {
        call_id: incoming.callId,
      });

      activeCall.value = {
        callId: incoming.callId,
        peerUserId: incoming.fromUserId,
        conversationId: incoming.conversationId,
        callType: incoming.callType,
        stunServers: incoming.stunServers,
        turnServers: incoming.turnServers,
      };

      incomingCall.value = null;
      callPhase.value = "connecting";

      await ensurePeerConnection(activeCall.value);
    } catch (error) {
      callError.value = toErrorMessage(error);
      throw error;
    }
  }

  async function rejectIncomingCall(reason = "decline"): Promise<void> {
    if (!incomingCall.value || !wsClient.value) {
      return;
    }

    try {
      await wsClient.value.sendSignaling("reject", {
        call_id: incomingCall.value.callId,
        reason,
      });
    } finally {
      incomingCall.value = null;
      callPhase.value = "idle";
    }
  }

  async function hangupCall(): Promise<void> {
    if (!activeCall.value || !wsClient.value) {
      resetCallState();
      return;
    }

    try {
      await wsClient.value.sendSignaling("hangup", {
        call_id: activeCall.value.callId,
      });
    } finally {
      resetCallState();
    }
  }

  async function handleSignalingEvent(event: SignalingPayload): Promise<void> {
    if (!event?.action) {
      return;
    }

    switch (event.action) {
      case "incoming_call": {
        if (callPhase.value !== "idle" && wsClient.value) {
          await wsClient.value.sendSignaling("reject", {
            call_id: event.call_id,
            reason: "busy",
          });
          return;
        }

        incomingCall.value = {
          callId: String(event.call_id || ""),
          fromUserId: Number(event.from_user || 0),
          conversationId: Number(event.payload?.conversation_id || 0),
          callType: (event.payload?.call_type || "audio") as CallMediaType,
          stunServers: event.payload?.stun_servers || [],
          turnServers: event.payload?.turn_servers || [],
        };
        callPhase.value = "incoming";
        return;
      }

      case "call_accepted": {
        if (!activeCall.value || activeCall.value.callId !== event.call_id) {
          return;
        }

        await startOffer(activeCall.value);
        return;
      }

      case "call_rejected":
      case "call_timeout":
      case "call_ended": {
        callError.value =
          event.action === "call_rejected"
            ? `通话被拒绝${event.payload?.reason ? `: ${event.payload.reason}` : ""}`
            : event.action === "call_timeout"
              ? "通话超时"
              : "对方已挂断";
        resetCallState();
        return;
      }

      case "sdp": {
        await handleSdpSignal(event);
        return;
      }

      case "ice_candidate": {
        await handleIceCandidateSignal(event);
        return;
      }

      default:
        return;
    }
  }

  async function handleSdpSignal(event: SignalingPayload): Promise<void> {
    if (!event.payload?.sdp_type || !event.payload?.sdp) {
      return;
    }

    const sdpType = event.payload.sdp_type as "offer" | "answer";

    if (!activeCall.value) {
      if (incomingCall.value && event.call_id === incomingCall.value.callId) {
        activeCall.value = {
          callId: incomingCall.value.callId,
          peerUserId: incomingCall.value.fromUserId,
          conversationId: incomingCall.value.conversationId,
          callType: incomingCall.value.callType,
          stunServers: incomingCall.value.stunServers,
          turnServers: incomingCall.value.turnServers,
        };
      } else {
        return;
      }
    }

    const call = activeCall.value;
    if (!call) {
      return;
    }

    const connection = await ensurePeerConnection(call);

    if (sdpType === "offer") {
      await connection.setRemoteDescription({
        type: "offer",
        sdp: event.payload.sdp,
      });

      const answer = await connection.createAnswer();
      await connection.setLocalDescription(answer);

      await wsClient.value?.sendSignaling("answer", {
        call_id: call.callId,
        target_id: call.peerUserId,
        sdp: answer.sdp,
      });

      callPhase.value = "connecting";
      return;
    }

    await connection.setRemoteDescription({
      type: "answer",
      sdp: event.payload.sdp,
    });
    callPhase.value = "connected";
  }

  async function handleIceCandidateSignal(event: SignalingPayload): Promise<void> {
    if (!peerConnection.value || !event.payload?.candidate) {
      return;
    }

    try {
      await peerConnection.value.addIceCandidate({
        candidate: event.payload.candidate,
        sdpMid: event.payload.sdp_mid,
        sdpMLineIndex: event.payload.sdp_mline_index,
      });
    } catch {
      // ignore invalid candidate
    }
  }

  function resetCallState(): void {
    callPhase.value = "idle";
    incomingCall.value = null;
    activeCall.value = null;

    if (peerConnection.value) {
      peerConnection.value.ontrack = null;
      peerConnection.value.onicecandidate = null;
      peerConnection.value.close();
      peerConnection.value = null;
    }

    stopLocalStream();
    stopRemoteStream();
  }

  function formatConversationSubtitle(conversationId: number): string {
    const messages = messageMap.value[conversationId] || [];
    if (!messages.length) {
      return "暂无消息";
    }

    const latest = messages[messages.length - 1];
    if (latest.revoked) {
      return "消息已撤回";
    }

    if (latest.kind === "text") {
      return latest.text || "文本消息";
    }

    if (latest.kind === "image") {
      return "[图片]";
    }

    if (latest.kind === "audio") {
      return "[语音]";
    }

    if (latest.kind === "video") {
      return "[视频]";
    }

    if (latest.kind === "file") {
      return "[文件]";
    }

    if (latest.kind === "call") {
      return "[通话消息]";
    }

    return "[消息]";
  }

  return {
    initializing,
    loading,
    wsConnected,
    wsError,
    errorMessage,
    conversations,
    contacts,
    presenceMap,
    unreadMap,
    readMap,
    messageMap,
    activeConversationId,
    activeConversation,
    activeMessages,
    sending,
    uploading,
    callPhase,
    incomingCall,
    activeCall,
    callError,
    localStream,
    remoteStream,
    isCalling,
    initialize,
    resetState,
    loadConversations,
    loadContacts,
    refreshPresence,
    openConversation,
    loadHistory,
    createConversation,
    refreshConversationDetail,
    renameConversation,
    startSingleChatWithUser,
    sendTextMessage,
    sendFileMessage,
    revokeMessage,
    markConversationRead,
    applyContact,
    handleContact,
    deleteContact,
    connectRealtime,
    disconnectRealtime,
    formatReadTip,
    formatConversationSubtitle,
    toFileURL,
    initiateCall,
    acceptIncomingCall,
    rejectIncomingCall,
    hangupCall,
  };
});
