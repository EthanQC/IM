<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { storeToRefs } from "pinia";
import { useRouter } from "vue-router";
import type { ConversationBrief, NormalizedMessage, UserBrief } from "../types/im";
import { formatMessageTime } from "../utils/message";
import { toErrorMessage } from "../services/response";
import { formatIMCode, resolveUserFromIdentifier } from "../utils/account";
import { useAuthStore } from "../stores/auth";
import { useIMStore } from "../stores/im";

type LeftMode = "chats" | "contacts";
type ToastType = "success" | "error" | "info";

interface ToastItem {
  id: string;
  type: ToastType;
  text: string;
}

interface PreviewImage {
  url: string;
  name: string;
}

const AVATAR_COLORS = [
  ["#fda4af", "#e11d48"],
  ["#f9a8d4", "#be185d"],
  ["#c4b5fd", "#7c3aed"],
  ["#a5b4fc", "#4338ca"],
  ["#93c5fd", "#1d4ed8"],
  ["#86efac", "#15803d"],
  ["#fcd34d", "#b45309"],
  ["#fdba74", "#c2410c"],
];

function avatarColor(name: string): [string, string] {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = ((hash << 5) - hash + name.charCodeAt(i)) | 0;
  }
  return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length];
}

const router = useRouter();
const auth = useAuthStore();
const im = useIMStore();

const {
  initializing,
  loading,
  wsConnected,
  errorMessage,
  conversations,
  contacts,
  presenceMap,
  unreadMap,
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
  messageMap,
} = storeToRefs(im);

const { profile, userId } = storeToRefs(auth);

const leftMode = ref<LeftMode>("chats");
const searchKeyword = ref("");
const messageInput = ref("");
const dragOver = ref(false);
const showConversationInfo = ref(false);
const showActionMenu = ref(false);

const fileInputEl = ref<HTMLInputElement | null>(null);
const messageScrollEl = ref<HTMLDivElement | null>(null);
const actionMenuEl = ref<HTMLElement | null>(null);

const localVideoEl = ref<HTMLVideoElement | null>(null);
const remoteVideoEl = ref<HTMLVideoElement | null>(null);
const remoteAudioEl = ref<HTMLAudioElement | null>(null);

const syncTimer = ref<number | null>(null);
const toasts = ref<ToastItem[]>([]);
const imagePreview = ref<PreviewImage | null>(null);
const conversationPeerMap = reactive<Record<number, UserBrief>>({});

const showAddFriendModal = ref(false);
const showHandleApplyModal = ref(false);
const showNewChatModal = ref(false);
const showProfileModal = ref(false);

const addFriendForm = reactive({
  identifier: "",
  remark: "",
});

const handleApplyForm = reactive({
  identifier: "",
  decision: "accept" as "accept" | "reject",
});

const newChatForm = reactive({
  mode: "single" as "single" | "group",
  singleIdentifier: "",
  groupTitle: "",
  groupMembers: "",
});

const profileForm = reactive({
  displayName: "",
  avatarURL: "",
});

const conversationTitleForm = reactive({
  title: "",
});

const currentUserName = computed(() => profile.value?.user?.display_name || profile.value?.user?.username || "用户");
const currentUserAvatar = computed(() => profile.value?.user?.avatar_url || "");
const myAccount = computed(() => (profile.value?.user?.username ? `@${profile.value.user.username}` : "-"));
const myIMCode = computed(() => formatIMCode(userId.value || 0) || "-");
const connectionLabel = computed(() => (wsConnected.value ? "在线" : "离线"));

const activeReadTip = computed(() => {
  if (!activeConversationId.value) {
    return "";
  }
  return im.formatReadTip(activeConversationId.value);
});

const activeConversationTitle = computed(() => {
  if (!activeConversation.value) {
    return "请选择会话";
  }
  return getConversationTitle(activeConversation.value);
});

const activeCallTargetId = computed<number | null>(() => {
  const conversation = activeConversation.value;
  if (!conversation || conversation.type !== 1) {
    return null;
  }

  const directPeer = conversationPeerMap[conversation.id];
  if (directPeer) {
    return directPeer.id;
  }

  const list = messageMap.value[conversation.id] || [];
  const peerMessage = [...list].reverse().find((item) => item.senderId !== userId.value);
  return peerMessage?.senderId || null;
});

const activePeer = computed<UserBrief | null>(() => {
  if (!activeConversation.value || activeConversation.value.type !== 1) {
    return null;
  }

  const byConversation = conversationPeerMap[activeConversation.value.id];
  if (byConversation) {
    return byConversation;
  }

  if (!activeCallTargetId.value) {
    return null;
  }

  return contacts.value.find((item) => item.id === activeCallTargetId.value) || null;
});

const callPhaseText = computed(() => {
  switch (callPhase.value) {
    case "incoming":
      return "来电中";
    case "outgoing":
      return "呼叫中";
    case "connecting":
      return "连接中";
    case "connected":
      return "通话中";
    default:
      return "空闲";
  }
});

const filteredConversations = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase();
  const baseIndex = new Map<number, number>();
  conversations.value.forEach((item, index) => baseIndex.set(item.id, index));

  const sorted = [...conversations.value].sort((a, b) => {
    const at = latestMessageUnix(a.id);
    const bt = latestMessageUnix(b.id);
    if (at !== bt) {
      return bt - at;
    }
    return (baseIndex.get(a.id) || 0) - (baseIndex.get(b.id) || 0);
  });

  if (!keyword) {
    return sorted;
  }

  return sorted.filter((item) => {
    const title = getConversationTitle(item).toLowerCase();
    const preview = conversationPreview(item).toLowerCase();
    return title.includes(keyword) || preview.includes(keyword);
  });
});

const filteredContacts = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase();
  if (!keyword) {
    return contacts.value;
  }

  return contacts.value.filter((item) => {
    const displayName = (item.display_name || "").toLowerCase();
    const username = (item.username || "").toLowerCase();
    const imCode = formatIMCode(item.id).toLowerCase();
    return displayName.includes(keyword) || username.includes(keyword) || imCode.includes(keyword);
  });
});

function pushToast(type: ToastType, text: string): void {
  if (!text) {
    return;
  }

  const id = `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
  toasts.value = [...toasts.value, { id, type, text }];

  window.setTimeout(() => {
    toasts.value = toasts.value.filter((item) => item.id !== id);
  }, 2600);
}

function latestMessageUnix(conversationId: number): number {
  const list = messageMap.value[conversationId] || [];
  return list[list.length - 1]?.createdAtUnix || 0;
}

function hasUsableURL(url: string): boolean {
  return /^https?:\/\//.test(url);
}

function avatarLetter(name: string): string {
  const normalized = (name || "?").trim();
  return (normalized[0] || "?").toUpperCase();
}

function userLabel(user: UserBrief | null | undefined): string {
  if (!user) {
    return "未命名联系人";
  }
  return user.display_name || user.username || "未命名联系人";
}

function userLabelById(targetId: number): string {
  const hit = contacts.value.find((item) => item.id === targetId);
  if (hit) {
    return userLabel(hit);
  }
  const code = formatIMCode(targetId);
  return code || "未知用户";
}

function isOnline(targetId: number): boolean {
  return Boolean(presenceMap.value[targetId]?.online);
}

function getConversationTitle(conversation: ConversationBrief): string {
  if (conversation.title?.trim()) {
    return conversation.title.trim();
  }

  if (conversation.type === 2) {
    return "群聊";
  }

  const peer = conversationPeerMap[conversation.id];
  if (peer) {
    return userLabel(peer);
  }

  return "单聊";
}

function conversationPreview(conversation: ConversationBrief): string {
  return im.formatConversationSubtitle(conversation.id) || "暂无消息";
}

function conversationTime(conversation: ConversationBrief): string {
  const unix = latestMessageUnix(conversation.id);
  if (!unix) {
    return "";
  }
  return formatMessageTime(unix);
}

function senderName(senderID: number): string {
  if (senderID === userId.value) {
    return "我";
  }

  const hit = contacts.value.find((item) => item.id === senderID);
  if (hit) {
    return userLabel(hit);
  }

  return userLabelById(senderID);
}

function messageAvatar(message: NormalizedMessage): string {
  if (message.senderId === userId.value) {
    return profile.value?.user?.avatar_url || "";
  }

  const hit = contacts.value.find((item) => item.id === message.senderId);
  return hit?.avatar_url || "";
}

function mediaURL(message: NormalizedMessage): string {
  if (!message.media?.object_key) {
    return "";
  }
  return im.toFileURL(message.media.object_key);
}

function messageFileName(message: NormalizedMessage): string {
  return message.media?.filename || "附件";
}

function isMine(message: NormalizedMessage): boolean {
  return message.senderId === userId.value;
}

function shouldShowSender(message: NormalizedMessage): boolean {
  return !isMine(message) && activeConversation.value?.type === 2;
}

function canRevoke(message: NormalizedMessage): boolean {
  return isMine(message) && !message.revoked && message.id > 0;
}

function bubbleClass(message: NormalizedMessage): Record<string, boolean> {
  return {
    mine: isMine(message),
    revoked: message.revoked,
    media: message.kind === "image" || message.kind === "video",
  };
}

function resolveIdentifierToUserId(identifier: string): number {
  const result = resolveUserFromIdentifier(identifier, contacts.value, profile.value?.user || null);
  if (!result.userId) {
    throw new Error("未找到该账号，请输入 IM 号或数字用户 ID");
  }
  return result.userId;
}

function parseIdentifierList(raw: string): number[] {
  const fields = raw
    .split(/[，,\n\s]+/)
    .map((item) => item.trim())
    .filter(Boolean);

  const ids: number[] = [];
  fields.forEach((field) => {
    const id = resolveIdentifierToUserId(field);
    if (id > 0 && id !== userId.value && !ids.includes(id)) {
      ids.push(id);
    }
  });

  return ids;
}

function closeAllDialogs(): void {
  showActionMenu.value = false;
  showAddFriendModal.value = false;
  showHandleApplyModal.value = false;
  showNewChatModal.value = false;
  showProfileModal.value = false;
}

function toggleActionMenu(): void {
  showActionMenu.value = !showActionMenu.value;
}

function openAddFriendDialog(): void {
  closeAllDialogs();
  showAddFriendModal.value = true;
}

function openHandleApplyDialog(): void {
  closeAllDialogs();
  showHandleApplyModal.value = true;
}

function openNewChatDialog(): void {
  closeAllDialogs();
  showNewChatModal.value = true;
}

function openProfileDialog(): void {
  closeAllDialogs();
  showProfileModal.value = true;
}

function openConversationInfo(): void {
  showConversationInfo.value = true;
  conversationTitleForm.title = activeConversation.value?.title || "";
}

function closeConversationInfo(): void {
  showConversationInfo.value = false;
}

async function selectConversation(conversationId: number): Promise<void> {
  await im.openConversation(conversationId);
  await nextTick();
  scrollToBottom(true);
}

async function refreshEverything(silent = false): Promise<void> {
  await Promise.all([im.loadConversations(), im.loadContacts()]);
  await im.refreshPresence();

  if (activeConversationId.value) {
    await im.loadHistory(activeConversationId.value);
  }

  if (!silent) {
    pushToast("success", "数据已刷新");
  }
}

function onComposerKeydown(event: KeyboardEvent): void {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    void submitTextMessage();
  }
}

async function submitTextMessage(): Promise<void> {
  const content = messageInput.value.trim();
  if (!content) {
    return;
  }

  try {
    await im.sendTextMessage(content);
    messageInput.value = "";
    await nextTick();
    scrollToBottom(true);
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function sendFile(file: File, successTip = "文件已发送"): Promise<void> {
  try {
    await im.sendFileMessage(file);
    await nextTick();
    scrollToBottom(true);
    pushToast("success", successTip);
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

function openFilePicker(): void {
  fileInputEl.value?.click();
}

async function onFileSelected(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement;
  const files = input.files ? Array.from(input.files) : [];
  if (!files.length) {
    return;
  }

  for (const file of files) {
    await sendFile(file);
  }

  input.value = "";
}

async function onComposerPaste(event: ClipboardEvent): Promise<void> {
  const items = event.clipboardData?.items;
  if (!items || !items.length) {
    return;
  }

  const imageItem = Array.from(items).find((item) => item.type.startsWith("image/"));
  if (!imageItem) {
    return;
  }

  const imageFile = imageItem.getAsFile();
  if (!imageFile) {
    return;
  }

  event.preventDefault();

  const ext = imageFile.type.split("/")[1] || "png";
  const filename = `paste-${Date.now()}.${ext}`;
  const file = new File([imageFile], filename, {
    type: imageFile.type || "image/png",
  });

  await sendFile(file, "图片已发送");
}

function onDragOver(event: DragEvent): void {
  event.preventDefault();
  dragOver.value = true;
}

function onDragLeave(event: DragEvent): void {
  event.preventDefault();
  if (!event.relatedTarget) {
    dragOver.value = false;
  }
}

async function onDropFile(event: DragEvent): Promise<void> {
  event.preventDefault();
  dragOver.value = false;

  const files = event.dataTransfer?.files ? Array.from(event.dataTransfer.files) : [];
  if (!files.length) {
    return;
  }

  for (const file of files) {
    await sendFile(file);
  }
}

async function revokeMessage(messageId: number): Promise<void> {
  try {
    await im.revokeMessage(messageId);
    pushToast("success", "消息已撤回");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

function previewMessageImage(message: NormalizedMessage): void {
  const url = mediaURL(message);
  if (!url) {
    return;
  }

  imagePreview.value = {
    url,
    name: messageFileName(message),
  };
}

function closeImagePreview(): void {
  imagePreview.value = null;
}

async function openChatFromContact(contact: UserBrief): Promise<void> {
  try {
    await im.startSingleChatWithUser(contact.id);
    if (activeConversationId.value) {
      conversationPeerMap[activeConversationId.value] = contact;
    }
    leftMode.value = "chats";
    pushToast("success", "会话已打开");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function removeContact(contact: UserBrief): Promise<void> {
  const confirmed = window.confirm(`确认删除联系人"${userLabel(contact)}"？`);
  if (!confirmed) {
    return;
  }

  try {
    await im.deleteContact(contact.id);
    pushToast("success", "联系人已删除");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function submitAddFriend(): Promise<void> {
  const identifier = addFriendForm.identifier.trim();
  if (!identifier) {
    pushToast("error", "请输入对方账号或 IM 号");
    return;
  }

  try {
    const targetUserId = resolveIdentifierToUserId(identifier);
    if (targetUserId === userId.value) {
      throw new Error("不能添加自己为联系人");
    }

    await im.applyContact(targetUserId, addFriendForm.remark.trim());
    await Promise.all([im.loadContacts(), im.refreshPresence()]);

    const alreadyContact = contacts.value.some((item) => item.id === targetUserId);
    pushToast("success", alreadyContact ? "你们已经是好友了" : "好友申请已发送");

    addFriendForm.identifier = "";
    addFriendForm.remark = "";
    showAddFriendModal.value = false;
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function submitHandleApply(): Promise<void> {
  const identifier = handleApplyForm.identifier.trim();
  if (!identifier) {
    pushToast("error", "请输入申请人账号或 IM 号");
    return;
  }

  try {
    const targetUserId = resolveIdentifierToUserId(identifier);
    if (targetUserId === userId.value) {
      throw new Error("不能处理自己");
    }

    const accept = handleApplyForm.decision === "accept";
    await im.handleContact(targetUserId, accept);
    await Promise.all([im.loadConversations(), im.loadContacts(), im.refreshPresence()]);

    pushToast("success", accept ? "已同意好友申请" : "已拒绝好友申请");
    handleApplyForm.identifier = "";
    showHandleApplyModal.value = false;
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function submitNewChat(): Promise<void> {
  try {
    if (newChatForm.mode === "single") {
      const identifier = newChatForm.singleIdentifier.trim();
      if (!identifier) {
        throw new Error("请输入对方账号或 IM 号");
      }

      const targetUserId = resolveIdentifierToUserId(identifier);
      if (targetUserId === userId.value) {
        throw new Error("不能和自己发起聊天");
      }

      await im.startSingleChatWithUser(targetUserId);
      const contact = contacts.value.find((item) => item.id === targetUserId);
      if (activeConversationId.value && contact) {
        conversationPeerMap[activeConversationId.value] = contact;
      }
      pushToast("success", "单聊已创建");
    } else {
      if (!newChatForm.groupMembers.trim()) {
        throw new Error("请输入群成员账号或 IM 号");
      }

      const memberIDs = parseIdentifierList(newChatForm.groupMembers);
      if (!memberIDs.length) {
        throw new Error("至少需要一个有效成员");
      }

      await im.createConversation({
        type: 2,
        title: newChatForm.groupTitle.trim() || undefined,
        memberIDs,
      });
      pushToast("success", "群聊已创建");
    }

    newChatForm.singleIdentifier = "";
    newChatForm.groupTitle = "";
    newChatForm.groupMembers = "";
    showNewChatModal.value = false;
    leftMode.value = "chats";
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function saveProfile(): Promise<void> {
  try {
    await auth.updateProfile({
      display_name: profileForm.displayName.trim() || undefined,
      avatar_url: profileForm.avatarURL.trim() || undefined,
    });

    pushToast("success", "资料已更新");
    showProfileModal.value = false;
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function saveConversationTitle(): Promise<void> {
  if (!activeConversation.value || activeConversation.value.type !== 2) {
    return;
  }

  const title = conversationTitleForm.title.trim();
  if (!title) {
    pushToast("error", "群聊名称不能为空");
    return;
  }

  try {
    await im.renameConversation(activeConversation.value.id, title);
    pushToast("success", "群聊名称已更新");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function startCall(type: "audio" | "video"): Promise<void> {
  if (!activeCallTargetId.value) {
    pushToast("error", "当前会话无法识别呼叫对象");
    return;
  }

  try {
    await im.initiateCall(activeCallTargetId.value, type);
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function acceptCall(): Promise<void> {
  try {
    await im.acceptIncomingCall();
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function rejectCall(): Promise<void> {
  try {
    await im.rejectIncomingCall();
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function hangupCall(): Promise<void> {
  try {
    await im.hangupCall();
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function logout(): Promise<void> {
  closeAllDialogs();
  auth.logout();
  im.resetState();
  await router.replace({ name: "login" });
}

function scrollToBottom(force = false): void {
  const el = messageScrollEl.value;
  if (!el) {
    return;
  }

  const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 120;
  if (force || nearBottom) {
    el.scrollTop = el.scrollHeight;
  }
}

function onGlobalClick(event: MouseEvent): void {
  if (!showActionMenu.value) {
    return;
  }

  const target = event.target as Node | null;
  if (!target || !actionMenuEl.value || actionMenuEl.value.contains(target)) {
    return;
  }

  showActionMenu.value = false;
}

watch(
  profile,
  (nextProfile) => {
    profileForm.displayName = nextProfile?.user?.display_name || "";
    profileForm.avatarURL = nextProfile?.user?.avatar_url || "";
  },
  { immediate: true },
);

watch(
  activeConversation,
  (nextConversation) => {
    if (!nextConversation) {
      showConversationInfo.value = false;
      return;
    }
    conversationTitleForm.title = nextConversation.title || "";
  },
  { immediate: true },
);

watch([localStream, localVideoEl], ([stream]) => {
  if (localVideoEl.value) {
    localVideoEl.value.srcObject = stream || null;
  }
});

watch([remoteStream, remoteVideoEl], ([stream]) => {
  if (remoteVideoEl.value) {
    remoteVideoEl.value.srcObject = stream || null;
  }
});

watch([remoteStream, remoteAudioEl], ([stream]) => {
  if (remoteAudioEl.value) {
    remoteAudioEl.value.srcObject = stream || null;
  }
});

watch(
  () => [activeConversationId.value, activeMessages.value.length],
  async () => {
    await nextTick();
    scrollToBottom(true);
  },
);

watch([activeConversationId, activeMessages, contacts], () => {
  if (!activeConversationId.value) {
    return;
  }

  if (conversationPeerMap[activeConversationId.value]) {
    return;
  }

  const peerMessage = [...activeMessages.value].reverse().find((item) => item.senderId !== userId.value);
  if (!peerMessage) {
    return;
  }

  const hit = contacts.value.find((item) => item.id === peerMessage.senderId);
  if (hit) {
    conversationPeerMap[activeConversationId.value] = hit;
  }
});

watch(
  errorMessage,
  (nextError, prevError) => {
    if (nextError && nextError !== prevError) {
      pushToast("error", nextError);
    }
  },
  { flush: "post" },
);

watch(callError, (nextError, prevError) => {
  if (nextError && nextError !== prevError) {
    pushToast("error", nextError);
  }
});

watch(incomingCall, (call) => {
  if (call) {
    pushToast("info", `${userLabelById(call.fromUserId)} 发起了${call.callType === "video" ? "视频" : "语音"}通话`);
  }
});

onMounted(async () => {
  document.addEventListener("click", onGlobalClick);

  try {
    await im.initialize();
    await nextTick();
    scrollToBottom(true);

    syncTimer.value = window.setInterval(() => {
      void im.loadConversations();
      void im.loadContacts();
      if (leftMode.value === "contacts") {
        void im.refreshPresence();
      }
    }, 15000);
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
});

onBeforeUnmount(() => {
  document.removeEventListener("click", onGlobalClick);
  if (syncTimer.value) {
    window.clearInterval(syncTimer.value);
    syncTimer.value = null;
  }
  im.disconnectRealtime();
});
</script>

<template>
  <main class="wx-app">
    <section class="wx-shell">
      <!-- Left nav rail -->
      <nav class="wx-nav-rail">
        <button class="nav-avatar" @click="openProfileDialog" title="我的资料">
          <img v-if="hasUsableURL(currentUserAvatar)" :src="currentUserAvatar" alt="我的头像" />
          <span v-else>{{ avatarLetter(currentUserName) }}</span>
        </button>

        <button :class="{ active: leftMode === 'chats' }" @click="leftMode = 'chats'" title="聊天">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
        </button>
        <button :class="{ active: leftMode === 'contacts' }" @click="leftMode = 'contacts'" title="通讯录">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
        </button>
        <button @click="openNewChatDialog" title="新建聊天">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        </button>

        <div class="nav-spacer"></div>
        <button @click="refreshEverything()" title="刷新">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>
        </button>
        <button @click="logout" title="退出登录">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
        </button>
      </nav>

      <!-- Sidebar -->
      <aside class="wx-sidebar">
        <div class="me-card">
          <div v-if="hasUsableURL(currentUserAvatar)" class="avatar avatar-lg">
            <img :src="currentUserAvatar" alt="我的头像" />
          </div>
          <div
            v-else
            class="avatar avatar-lg avatar-fallback"
            :style="{ background: `linear-gradient(135deg, ${avatarColor(currentUserName)[0]}, ${avatarColor(currentUserName)[1]})` }"
          >
            {{ avatarLetter(currentUserName) }}
          </div>

          <div class="me-meta">
            <h2>{{ currentUserName }}</h2>
            <p>{{ myAccount }}</p>
          </div>

          <span class="status-chip" :class="{ online: wsConnected }">
            <span class="status-dot"></span>
            {{ connectionLabel }}
          </span>
        </div>

        <div class="sidebar-search">
          <div class="search-box">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
            <input v-model="searchKeyword" type="search" placeholder="搜索" />
          </div>
        </div>

        <div class="sidebar-tabs">
          <button :class="{ active: leftMode === 'chats' }" @click="leftMode = 'chats'">聊天</button>
          <button :class="{ active: leftMode === 'contacts' }" @click="leftMode = 'contacts'">通讯录</button>
        </div>

        <div class="sidebar-list">
          <template v-if="leftMode === 'chats'">
            <button
              v-for="conversation in filteredConversations"
              :key="conversation.id"
              class="chat-item"
              :class="{ active: conversation.id === activeConversationId }"
              @click="selectConversation(conversation.id)"
            >
              <div
                class="avatar avatar-md avatar-fallback"
                :style="{ background: `linear-gradient(135deg, ${avatarColor(getConversationTitle(conversation))[0]}, ${avatarColor(getConversationTitle(conversation))[1]})` }"
              >
                {{ avatarLetter(getConversationTitle(conversation)) }}
              </div>

              <div class="chat-item-main">
                <div class="chat-item-head">
                  <span class="chat-title">{{ getConversationTitle(conversation) }}</span>
                  <span class="chat-time">{{ conversationTime(conversation) }}</span>
                </div>
                <p class="chat-subtitle">{{ conversationPreview(conversation) }}</p>
              </div>

              <span v-if="unreadMap[conversation.id]" class="unread-badge">{{ unreadMap[conversation.id] }}</span>
            </button>

            <div v-if="!filteredConversations.length" class="empty-list">
              <p>暂无会话</p>
              <button class="link-action" @click="openNewChatDialog">发起新聊天</button>
            </div>
          </template>

          <template v-else>
            <article v-for="contact in filteredContacts" :key="contact.id" class="contact-item">
              <div
                class="avatar avatar-md"
                :class="{ 'avatar-fallback': !hasUsableURL(contact.avatar_url) }"
                :style="!hasUsableURL(contact.avatar_url) ? { background: `linear-gradient(135deg, ${avatarColor(userLabel(contact))[0]}, ${avatarColor(userLabel(contact))[1]})` } : undefined"
              >
                <img v-if="hasUsableURL(contact.avatar_url)" :src="contact.avatar_url" :alt="userLabel(contact)" />
                <span v-else>{{ avatarLetter(userLabel(contact)) }}</span>
              </div>

              <div class="contact-main" @click="openChatFromContact(contact)">
                <div class="contact-name-row">
                  <h3>{{ userLabel(contact) }}</h3>
                  <span class="contact-online" :class="{ online: isOnline(contact.id) }">
                    <span class="status-dot"></span>
                    {{ isOnline(contact.id) ? "在线" : "离线" }}
                  </span>
                </div>
                <p>{{ `@${contact.username}` }}</p>
              </div>

              <button class="danger-text" @click="removeContact(contact)">删除</button>
            </article>

            <div v-if="!filteredContacts.length" class="empty-list">
              <p>暂无联系人</p>
              <button class="link-action" @click="openAddFriendDialog">添加好友</button>
            </div>
          </template>
        </div>

        <div class="sidebar-actions">
          <button @click="openNewChatDialog">新建聊天</button>
          <button @click="openAddFriendDialog">添加好友</button>
        </div>
      </aside>

      <!-- Main chat area -->
      <section class="wx-main">
        <header class="chat-topbar">
          <div class="chat-topbar-title">
            <h1>{{ activeConversationTitle }}</h1>
            <p v-if="activeConversation">
              {{ activeReadTip || (activePeer ? (isOnline(activePeer.id) ? "对方在线" : "对方离线") : "") }}
            </p>
            <p v-else>选择一个会话开始聊天</p>
          </div>

          <div class="chat-topbar-actions">
            <button v-if="activeConversation && activeConversation.type === 1" class="topbar-btn" :disabled="Boolean(activeCall)" @click="startCall('audio')" title="语音通话">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z"/></svg>
            </button>
            <button v-if="activeConversation && activeConversation.type === 1" class="topbar-btn" :disabled="Boolean(activeCall)" @click="startCall('video')" title="视频通话">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="23 7 16 12 23 17 23 7"/><rect x="1" y="5" width="15" height="14" rx="2" ry="2"/></svg>
            </button>
            <button class="topbar-btn" :disabled="!activeConversation" @click="openConversationInfo" title="会话信息">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
            </button>

            <div ref="actionMenuEl" class="menu-wrap">
              <button class="topbar-btn" @click.stop="toggleActionMenu" title="更多">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/></svg>
              </button>

              <Transition name="menu-fade">
                <div v-if="showActionMenu" class="action-menu" @click.stop>
                  <button @click="openNewChatDialog">发起聊天</button>
                  <button @click="openAddFriendDialog">添加好友</button>
                  <button @click="openHandleApplyDialog">处理好友申请</button>
                  <button @click="openProfileDialog">编辑资料</button>
                  <button @click="refreshEverything()">刷新数据</button>
                  <div class="menu-divider"></div>
                  <button class="danger-text" @click="logout">退出登录</button>
                </div>
              </Transition>
            </div>
          </div>
        </header>

        <!-- Chat body with messages -->
        <div
          v-if="activeConversation"
          class="chat-body"
          @dragover="onDragOver"
          @dragleave="onDragLeave"
          @drop="onDropFile"
        >
          <div ref="messageScrollEl" class="message-list">
            <article
              v-for="message in activeMessages"
              :key="`${message.id}-${message.seq}`"
              class="msg-row"
              :class="{ mine: isMine(message) }"
            >
              <div class="msg-avatar" :class="{ right: isMine(message) }">
                <div v-if="hasUsableURL(messageAvatar(message))" class="avatar avatar-sm">
                  <img :src="messageAvatar(message)" alt="头像" />
                </div>
                <div
                  v-else
                  class="avatar avatar-sm avatar-fallback"
                  :style="{ background: `linear-gradient(135deg, ${avatarColor(senderName(message.senderId))[0]}, ${avatarColor(senderName(message.senderId))[1]})` }"
                >
                  {{ avatarLetter(senderName(message.senderId)) }}
                </div>
              </div>

              <div class="msg-main">
                <p v-if="shouldShowSender(message)" class="msg-sender">{{ senderName(message.senderId) }}</p>

                <section class="msg-bubble" :class="bubbleClass(message)">
                  <template v-if="message.revoked">
                    <span class="revoked-text">{{ isMine(message) ? "你撤回了一条消息" : "对方撤回了一条消息" }}</span>
                  </template>

                  <template v-else-if="message.kind === 'text'">
                    <p class="msg-text">{{ message.text }}</p>
                  </template>

                  <template v-else-if="message.kind === 'image'">
                    <img
                      class="msg-image"
                      :src="mediaURL(message)"
                      :alt="messageFileName(message)"
                      @click="previewMessageImage(message)"
                    />
                  </template>

                  <template v-else-if="message.kind === 'file'">
                    <a class="msg-file" :href="mediaURL(message)" target="_blank" rel="noreferrer">
                      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg>
                      <span class="file-name">{{ messageFileName(message) }}</span>
                    </a>
                  </template>

                  <template v-else-if="message.kind === 'audio'">
                    <audio controls :src="mediaURL(message)"></audio>
                  </template>

                  <template v-else-if="message.kind === 'video'">
                    <video class="msg-video" controls :src="mediaURL(message)"></video>
                  </template>

                  <template v-else-if="message.kind === 'call'">
                    <p class="msg-call">{{ message.callHint || "通话记录" }}</p>
                  </template>

                  <template v-else>
                    <p class="msg-text">暂不支持的消息类型</p>
                  </template>
                </section>

                <footer class="msg-meta">
                  <time>{{ formatMessageTime(message.createdAtUnix) }}</time>
                  <button v-if="canRevoke(message)" class="revoke-btn" @click="revokeMessage(message.id)">撤回</button>
                </footer>
              </div>
            </article>
          </div>

          <div v-if="dragOver" class="drop-mask">释放文件以发送</div>

          <!-- Composer -->
          <footer class="composer">
            <div class="composer-tools">
              <div class="composer-tool-left">
                <button class="tool-btn" :disabled="uploading" @click="openFilePicker" title="发送文件">
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg>
                </button>
              </div>
              <span class="composer-hint">Ctrl+V 粘贴图片 / 拖拽文件到此处</span>
            </div>

            <textarea
              v-model="messageInput"
              class="composer-input"
              rows="3"
              placeholder="输入消息，Enter 发送，Shift+Enter 换行"
              @keydown="onComposerKeydown"
              @paste="onComposerPaste"
            />

            <div class="composer-actions">
              <span v-if="sending || uploading" class="composer-state">{{ sending ? "发送中..." : "上传中..." }}</span>
              <button
                class="send-btn"
                :disabled="sending || uploading || !messageInput.trim()"
                @click="submitTextMessage"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
                发送
              </button>
            </div>
          </footer>
        </div>

        <!-- Empty state when no conversation is selected -->
        <section v-else class="chat-empty">
          <div class="empty-card">
            <div class="empty-icon">
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
            </div>
            <h2>开始聊天</h2>
            <p>从左侧选择一个会话，或创建新的聊天</p>
            <div class="empty-actions">
              <button class="primary-btn" @click="openNewChatDialog">发起聊天</button>
              <button class="outline-btn" @click="openAddFriendDialog">添加好友</button>
            </div>
          </div>
        </section>
      </section>

      <!-- Conversation info drawer -->
      <aside class="conversation-info" :class="{ show: showConversationInfo }">
        <header>
          <h3>会话信息</h3>
          <button class="topbar-btn" @click="closeConversationInfo">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </header>

        <template v-if="activeConversation">
          <section class="info-block profile-block">
            <div
              class="avatar avatar-lg avatar-fallback"
              :style="{ background: `linear-gradient(135deg, ${avatarColor(activeConversationTitle)[0]}, ${avatarColor(activeConversationTitle)[1]})` }"
            >
              {{ avatarLetter(activeConversationTitle) }}
            </div>
            <div>
              <h4>{{ activeConversationTitle }}</h4>
              <p class="info-type-badge">{{ activeConversation.type === 2 ? "群聊" : "单聊" }}</p>
            </div>
          </section>

          <section v-if="activePeer" class="info-block">
            <h5>联系人信息</h5>
            <div class="info-row"><span>昵称</span><span>{{ userLabel(activePeer) }}</span></div>
            <div class="info-row"><span>账号</span><span>{{ `@${activePeer.username}` }}</span></div>
            <div class="info-row"><span>IM 号</span><span>{{ formatIMCode(activePeer.id) }}</span></div>
            <div class="info-row"><span>状态</span><span :class="isOnline(activePeer.id) ? 'text-online' : ''">{{ isOnline(activePeer.id) ? "在线" : "离线" }}</span></div>
          </section>

          <section v-if="activeConversation.type === 2" class="info-block">
            <h5>群聊设置</h5>
            <label>
              <span>群聊名称</span>
              <input v-model="conversationTitleForm.title" type="text" placeholder="输入群聊名称" />
            </label>
            <button class="primary-btn-sm" @click="saveConversationTitle">保存名称</button>
          </section>

          <section class="info-block">
            <h5>快捷操作</h5>
            <div class="info-actions">
              <button @click="openAddFriendDialog">添加好友</button>
              <button @click="openHandleApplyDialog">处理申请</button>
              <button @click="openProfileDialog">编辑资料</button>
              <button @click="refreshEverything()">刷新数据</button>
            </div>
          </section>

          <section class="info-block">
            <h5>连接状态</h5>
            <div class="info-row"><span>实时连接</span><span :class="wsConnected ? 'text-online' : 'text-offline'">{{ wsConnected ? "正常" : "重连中" }}</span></div>
            <div class="info-row"><span>通话状态</span><span>{{ callPhaseText }}</span></div>
          </section>
        </template>
      </aside>
    </section>

    <!-- Hidden inputs -->
    <input ref="fileInputEl" type="file" multiple class="hidden-file" @change="onFileSelected" />
    <audio ref="remoteAudioEl" autoplay playsinline class="visually-hidden"></audio>

    <!-- Add Friend Modal -->
    <Transition name="modal-fade">
      <div v-if="showAddFriendModal" class="modal-mask" @click.self="showAddFriendModal = false">
        <section class="modal-card">
          <header>
            <h3>添加好友</h3>
            <button class="modal-close" @click="showAddFriendModal = false">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          </header>

          <label>
            <span>对方 IM 号 / 用户 ID</span>
            <input v-model="addFriendForm.identifier" type="text" placeholder="例如 IM-00001C 或 28" />
          </label>

          <label>
            <span>申请备注</span>
            <textarea v-model="addFriendForm.remark" rows="3" placeholder="你好，我是..." />
          </label>

          <footer>
            <button class="outline-btn" @click="showAddFriendModal = false">取消</button>
            <button class="primary-btn" @click="submitAddFriend">发送申请</button>
          </footer>
        </section>
      </div>
    </Transition>

    <!-- Handle Friend Apply Modal -->
    <Transition name="modal-fade">
      <div v-if="showHandleApplyModal" class="modal-mask" @click.self="showHandleApplyModal = false">
        <section class="modal-card">
          <header>
            <h3>处理好友申请</h3>
            <button class="modal-close" @click="showHandleApplyModal = false">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          </header>

          <label>
            <span>申请人 IM 号 / 用户 ID</span>
            <input v-model="handleApplyForm.identifier" type="text" placeholder="输入申请人的 IM 号或用户 ID" />
          </label>

          <label>
            <span>处理结果</span>
            <select v-model="handleApplyForm.decision">
              <option value="accept">同意</option>
              <option value="reject">拒绝</option>
            </select>
          </label>

          <p class="form-tip">请让对方将账号或 IM 号发给你再进行处理</p>

          <footer>
            <button class="outline-btn" @click="showHandleApplyModal = false">取消</button>
            <button class="primary-btn" @click="submitHandleApply">提交</button>
          </footer>
        </section>
      </div>
    </Transition>

    <!-- New Chat Modal -->
    <Transition name="modal-fade">
      <div v-if="showNewChatModal" class="modal-mask" @click.self="showNewChatModal = false">
        <section class="modal-card">
          <header>
            <h3>发起聊天</h3>
            <button class="modal-close" @click="showNewChatModal = false">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          </header>

          <div class="mode-switch">
            <button :class="{ active: newChatForm.mode === 'single' }" @click="newChatForm.mode = 'single'">单聊</button>
            <button :class="{ active: newChatForm.mode === 'group' }" @click="newChatForm.mode = 'group'">群聊</button>
          </div>

          <template v-if="newChatForm.mode === 'single'">
            <label>
              <span>对方 IM 号 / 用户 ID</span>
              <input v-model="newChatForm.singleIdentifier" type="text" placeholder="例如 IM-00001C 或 28" />
            </label>
          </template>

          <template v-else>
            <label>
              <span>群聊名称</span>
              <input v-model="newChatForm.groupTitle" type="text" placeholder="例如 项目讨论组" />
            </label>

            <label>
              <span>成员（逗号或换行分隔）</span>
              <textarea
                v-model="newChatForm.groupMembers"
                rows="4"
                placeholder="IM-00002A, 29&#10;IM-00002B"
              />
            </label>
          </template>

          <footer>
            <button class="outline-btn" @click="showNewChatModal = false">取消</button>
            <button class="primary-btn" @click="submitNewChat">创建</button>
          </footer>
        </section>
      </div>
    </Transition>

    <!-- Profile Modal -->
    <Transition name="modal-fade">
      <div v-if="showProfileModal" class="modal-mask" @click.self="showProfileModal = false">
        <section class="modal-card">
          <header>
            <h3>我的资料</h3>
            <button class="modal-close" @click="showProfileModal = false">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          </header>

          <label>
            <span>显示名</span>
            <input v-model="profileForm.displayName" type="text" placeholder="显示名" />
          </label>

          <label>
            <span>头像 URL</span>
            <input v-model="profileForm.avatarURL" type="url" placeholder="https://..." />
          </label>

          <section class="profile-summary">
            <div class="info-row"><span>账号</span><span>{{ myAccount }}</span></div>
            <div class="info-row"><span>IM 号</span><span>{{ myIMCode }}</span></div>
          </section>

          <footer>
            <button class="outline-btn" @click="showProfileModal = false">取消</button>
            <button class="primary-btn" @click="saveProfile">保存</button>
          </footer>
        </section>
      </div>
    </Transition>

    <!-- Incoming Call Modal -->
    <Transition name="modal-fade">
      <div v-if="incomingCall" class="modal-mask call-mask" @click.self="rejectCall">
        <section class="call-card">
          <div class="call-icon">
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z"/></svg>
          </div>
          <h3>{{ `${userLabelById(incomingCall.fromUserId)} 邀请你${incomingCall.callType === 'video' ? '视频通话' : '语音通话'}` }}</h3>
          <p>请在 30 秒内处理来电</p>
          <footer>
            <button class="danger-btn" @click="rejectCall">拒绝</button>
            <button class="accept-btn" @click="acceptCall">接听</button>
          </footer>
        </section>
      </div>
    </Transition>

    <!-- Active Call Panel -->
    <Transition name="call-slide">
      <div v-if="activeCall" class="active-call-panel">
        <header>
          <h4>{{ `${userLabelById(activeCall.peerUserId)} · ${activeCall.callType === 'video' ? '视频通话' : '语音通话'}` }}</h4>
          <span>{{ callPhaseText }}</span>
        </header>

        <section class="active-call-body">
          <video
            v-if="activeCall.callType === 'video'"
            ref="remoteVideoEl"
            autoplay
            playsinline
            class="remote-video"
          ></video>
          <div v-else class="audio-placeholder">
            <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z"/></svg>
            <span>语音通话进行中</span>
          </div>

          <video
            v-if="activeCall.callType === 'video'"
            ref="localVideoEl"
            autoplay
            muted
            playsinline
            class="local-video"
          ></video>
        </section>

        <footer>
          <button class="danger-btn" @click="hangupCall">挂断</button>
        </footer>
      </div>
    </Transition>

    <!-- Image Preview -->
    <Transition name="modal-fade">
      <div v-if="imagePreview" class="modal-mask" @click.self="closeImagePreview">
        <section class="preview-card">
          <img :src="imagePreview.url" :alt="imagePreview.name" />
          <footer>
            <a :href="imagePreview.url" target="_blank" rel="noreferrer">在新窗口打开</a>
            <button class="outline-btn" @click="closeImagePreview">关闭</button>
          </footer>
        </section>
      </div>
    </Transition>

    <!-- Toasts -->
    <div class="toast-stack">
      <TransitionGroup name="toast-slide">
        <article v-for="toast in toasts" :key="toast.id" class="toast" :class="toast.type">
          {{ toast.text }}
        </article>
      </TransitionGroup>
    </div>

    <!-- Global loading -->
    <Transition name="fade">
      <div v-if="initializing || loading" class="global-loading">
        <span class="loading-spinner"></span>
        同步中...
      </div>
    </Transition>
  </main>
</template>

<style scoped>
/* ==================== Layout ==================== */
.wx-app {
  --wx-bg: #fef7f9;
  --wx-surface: #ffffff;
  --wx-side: #fdf2f5;
  --wx-border: #f0d4dc;
  --wx-border-soft: #f5e0e7;
  --wx-text: #2d1f24;
  --wx-muted: #9e7a86;
  --wx-pink: #d4507a;
  --wx-pink-strong: #c03d68;
  --wx-pink-soft: #fbe8ef;
  --wx-pink-light: #fdf0f4;
  --wx-danger: #e54d4d;
  --wx-success: #16a34a;
  min-height: 100dvh;
  width: 100%;
  padding: 14px;
  background: var(--wx-bg);
}

.wx-shell {
  position: relative;
  display: grid;
  grid-template-columns: 64px minmax(260px, 320px) 1fr;
  height: calc(100dvh - 28px);
  border: 1px solid var(--wx-border);
  border-radius: 14px;
  overflow: hidden;
  background: var(--wx-surface);
  box-shadow: 0 8px 40px -10px rgba(180, 60, 100, 0.1);
}

/* ==================== Nav Rail ==================== */
.wx-nav-rail {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 12px 8px;
  border-right: 1px solid var(--wx-border);
  background: linear-gradient(180deg, #3d2b33, #2d1f28);
}

.wx-nav-rail button {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.6);
  cursor: pointer;
  display: grid;
  place-items: center;
  transition: background 0.15s, color 0.15s;
}

.wx-nav-rail button:hover {
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.9);
}

.wx-nav-rail button.active {
  background: linear-gradient(135deg, #d4507a, #c03d68);
  color: #fff;
  box-shadow: 0 4px 12px -2px rgba(212, 80, 122, 0.4);
}

.wx-nav-rail .nav-avatar {
  width: 44px;
  height: 44px;
  border-radius: 14px;
  background: linear-gradient(135deg, #f472b6, #d4507a);
  color: #fff;
  overflow: hidden;
  margin-bottom: 6px;
  font-weight: 700;
  font-size: 16px;
}

.wx-nav-rail .nav-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.nav-spacer {
  flex: 1;
}

/* ==================== Sidebar ==================== */
.wx-sidebar {
  display: grid;
  grid-template-rows: auto auto auto 1fr auto;
  border-right: 1px solid var(--wx-border);
  background: var(--wx-side);
}

.me-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 14px;
  border-bottom: 1px solid var(--wx-border);
}

.me-meta {
  min-width: 0;
  flex: 1;
}

.me-meta h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--wx-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.me-meta p {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--wx-muted);
}

.status-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border-radius: 999px;
  border: 1px solid var(--wx-border);
  color: var(--wx-muted);
  font-size: 11px;
  padding: 3px 10px 3px 8px;
  line-height: 16px;
  background: #fff;
  white-space: nowrap;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #ccc;
  flex-shrink: 0;
}

.status-chip.online {
  color: #15803d;
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.status-chip.online .status-dot {
  background: #22c55e;
}

.sidebar-search {
  padding: 10px 14px;
}

.search-box {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  border-radius: 8px;
  border: 1.5px solid var(--wx-border);
  background: #fff;
  padding: 0 10px;
  transition: border-color 0.2s, box-shadow 0.2s;
  color: var(--wx-muted);
}

.search-box:focus-within {
  border-color: #e88aab;
  box-shadow: 0 0 0 3px rgba(212, 80, 122, 0.08);
}

.search-box input {
  flex: 1;
  border: 0;
  background: transparent;
  font-size: 13px;
  padding: 0;
  color: var(--wx-text);
}

.search-box input:focus {
  outline: none;
}

.search-box input::placeholder {
  color: #c4a3ae;
}

.sidebar-tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
  padding: 0 14px 10px;
}

.sidebar-tabs button {
  height: 32px;
  border-radius: 8px;
  border: 1.5px solid var(--wx-border);
  background: #fff;
  font-size: 13px;
  color: var(--wx-muted);
  cursor: pointer;
  font-weight: 500;
  transition: all 0.15s;
}

.sidebar-tabs button.active {
  border-color: #f0a0bd;
  background: var(--wx-pink-soft);
  color: var(--wx-pink);
  font-weight: 600;
}

.sidebar-list {
  overflow: auto;
  padding: 2px 0 8px;
}

/* Conversation item */
.chat-item {
  width: 100%;
  border: 0;
  border-bottom: 1px solid var(--wx-border-soft);
  background: transparent;
  text-align: left;
  padding: 12px 14px;
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  transition: background 0.12s;
}

.chat-item:hover {
  background: rgba(212, 80, 122, 0.04);
}

.chat-item.active {
  background: var(--wx-pink-soft);
}

.chat-item-main {
  min-width: 0;
}

.chat-item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.chat-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--wx-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chat-time {
  font-size: 11px;
  color: #b8929e;
  white-space: nowrap;
}

.chat-subtitle {
  margin: 3px 0 0;
  color: var(--wx-muted);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.unread-badge {
  min-width: 20px;
  height: 20px;
  border-radius: 10px;
  background: var(--wx-pink);
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 600;
  padding: 0 6px;
}

/* Contact item */
.contact-item {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--wx-border-soft);
}

.contact-main {
  min-width: 0;
  cursor: pointer;
}

.contact-main h3 {
  margin: 0;
  font-size: 14px;
  color: var(--wx-text);
  font-weight: 600;
}

.contact-main p {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--wx-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.contact-name-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.contact-online {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: #b8929e;
}

.contact-online .status-dot {
  width: 5px;
  height: 5px;
}

.contact-online.online {
  color: #15803d;
}

.contact-online.online .status-dot {
  background: #22c55e;
}

.sidebar-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  padding: 12px 14px;
  border-top: 1px solid var(--wx-border);
}

.sidebar-actions button {
  height: 34px;
  border-radius: 8px;
  border: 1.5px solid var(--wx-border);
  background: #fff;
  color: var(--wx-text);
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: all 0.15s;
}

.sidebar-actions button:hover {
  border-color: #e88aab;
  background: var(--wx-pink-soft);
  color: var(--wx-pink);
}

.empty-list {
  padding: 32px 16px;
  text-align: center;
}

.empty-list p {
  margin: 0;
  color: var(--wx-muted);
  font-size: 13px;
}

.link-action {
  display: inline-block;
  margin-top: 8px;
  border: 0;
  background: transparent;
  color: var(--wx-pink);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  padding: 0;
}

.link-action:hover {
  color: var(--wx-pink-strong);
}

/* ==================== Main Chat Area ==================== */
.wx-main {
  display: grid;
  grid-template-rows: auto 1fr;
  background: var(--wx-pink-light);
}

.chat-topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  padding: 12px 18px;
  border-bottom: 1px solid var(--wx-border);
  background: rgba(255, 255, 255, 0.85);
  backdrop-filter: blur(8px);
}

.chat-topbar-title {
  min-width: 0;
}

.chat-topbar-title h1 {
  margin: 0;
  font-size: 17px;
  color: var(--wx-text);
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chat-topbar-title p {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--wx-muted);
}

.chat-topbar-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.topbar-btn {
  width: 34px;
  height: 34px;
  border-radius: 8px;
  border: 1px solid var(--wx-border);
  background: #fff;
  color: var(--wx-muted);
  cursor: pointer;
  display: grid;
  place-items: center;
  transition: all 0.15s;
}

.topbar-btn:hover:not(:disabled) {
  border-color: #e88aab;
  color: var(--wx-pink);
  background: var(--wx-pink-soft);
}

.topbar-btn:disabled {
  cursor: not-allowed;
  opacity: 0.4;
}

.menu-wrap {
  position: relative;
}

.action-menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 180px;
  border: 1px solid var(--wx-border);
  background: #fff;
  border-radius: 10px;
  box-shadow: 0 12px 36px rgba(180, 60, 100, 0.12);
  padding: 6px;
  z-index: 24;
}

.action-menu button {
  width: 100%;
  text-align: left;
  border: 0;
  border-radius: 6px;
  background: transparent;
  padding: 8px 10px;
  color: var(--wx-text);
  font-size: 13px;
  cursor: pointer;
  transition: background 0.12s;
}

.action-menu button:hover {
  background: var(--wx-pink-soft);
}

.menu-divider {
  height: 1px;
  background: var(--wx-border-soft);
  margin: 4px 6px;
}

/* ==================== Chat Body ==================== */
.chat-body {
  position: relative;
  display: grid;
  grid-template-rows: 1fr auto;
  min-height: 0;
}

.message-list {
  overflow: auto;
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  scroll-behavior: smooth;
}

.msg-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px;
  align-items: flex-start;
  max-width: min(80%, 700px);
}

.msg-row.mine {
  margin-left: auto;
  grid-template-columns: minmax(0, 1fr) auto;
}

.msg-avatar.right {
  order: 2;
}

.msg-main {
  min-width: 0;
}

.msg-sender {
  margin: 0 0 3px;
  font-size: 12px;
  color: var(--wx-muted);
}

.msg-bubble {
  display: inline-block;
  max-width: 100%;
  border-radius: 12px;
  padding: 10px 14px;
  background: #fff;
  border: 1px solid var(--wx-border-soft);
  color: var(--wx-text);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
}

.msg-bubble.mine {
  background: linear-gradient(135deg, #fce4ec, #f8bbd0);
  border-color: #f0a0bd;
}

.msg-bubble.media {
  padding: 4px;
  background: transparent;
  border: none;
  box-shadow: none;
}

.msg-bubble.mine.media {
  background: transparent;
  border: none;
}

.msg-bubble.revoked {
  background: #fdf2f5;
  border-color: var(--wx-border-soft);
  box-shadow: none;
}

.msg-text {
  margin: 0;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 14px;
}

.msg-image {
  display: block;
  max-width: min(320px, 60vw);
  border-radius: 10px;
  cursor: zoom-in;
}

.msg-video {
  width: min(340px, 65vw);
  border-radius: 10px;
  background: #000;
}

.msg-file {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--wx-pink);
  text-decoration: none;
  word-break: break-all;
  font-size: 14px;
}

.msg-file:hover {
  color: var(--wx-pink-strong);
}

.file-name {
  text-decoration: underline;
  text-underline-offset: 2px;
}

.msg-call {
  margin: 0;
  color: var(--wx-muted);
  font-size: 14px;
}

.revoked-text {
  color: var(--wx-muted);
  font-size: 13px;
  font-style: italic;
}

.msg-meta {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}

.msg-row:not(.mine) .msg-meta {
  justify-content: flex-start;
}

.msg-meta time {
  font-size: 11px;
  color: #b8929e;
}

.revoke-btn {
  border: 0;
  background: transparent;
  color: #b8929e;
  font-size: 11px;
  cursor: pointer;
  padding: 0;
  transition: color 0.15s;
}

.revoke-btn:hover {
  color: var(--wx-pink);
}

.drop-mask {
  position: absolute;
  inset: 0;
  background: rgba(212, 80, 122, 0.06);
  border: 2px dashed rgba(212, 80, 122, 0.3);
  border-radius: 8px;
  color: var(--wx-pink);
  font-weight: 600;
  font-size: 15px;
  display: grid;
  place-items: center;
  pointer-events: none;
  z-index: 5;
}

/* ==================== Composer ==================== */
.composer {
  border-top: 1px solid var(--wx-border);
  background: #fff;
  padding: 10px 16px 12px;
}

.composer-tools {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 6px;
}

.composer-tool-left {
  display: flex;
  align-items: center;
  gap: 4px;
}

.tool-btn {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  border: 0;
  background: transparent;
  color: var(--wx-muted);
  cursor: pointer;
  display: grid;
  place-items: center;
  transition: all 0.12s;
}

.tool-btn:hover:not(:disabled) {
  background: var(--wx-pink-soft);
  color: var(--wx-pink);
}

.tool-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.composer-hint {
  font-size: 11px;
  color: #c4a3ae;
}

.composer-input {
  width: 100%;
  resize: none;
  min-height: 72px;
  border: 0;
  border-radius: 8px;
  padding: 8px 0;
  background: #fff;
  font-size: 14px;
  line-height: 1.6;
  color: var(--wx-text);
}

.composer-input:focus {
  outline: none;
}

.composer-input::placeholder {
  color: #c4a3ae;
}

.composer-actions {
  margin-top: 4px;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
}

.composer-state {
  font-size: 12px;
  color: var(--wx-muted);
}

.send-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 80px;
  height: 34px;
  border: 0;
  border-radius: 8px;
  background: linear-gradient(135deg, #f472b6, #d4507a);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  padding: 0 14px;
  transition: opacity 0.15s;
  box-shadow: 0 2px 8px -2px rgba(212, 80, 122, 0.3);
  justify-content: center;
}

.send-btn:hover:not(:disabled) {
  opacity: 0.9;
}

.send-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

/* ==================== Empty State ==================== */
.chat-empty {
  display: grid;
  place-items: center;
  padding: 18px;
}

.empty-card {
  width: min(400px, 92%);
  border: 1px solid var(--wx-border);
  border-radius: 16px;
  background: #fff;
  padding: 40px 30px;
  text-align: center;
}

.empty-icon {
  color: #e88aab;
  margin-bottom: 16px;
}

.empty-card h2 {
  margin: 0;
  color: var(--wx-text);
  font-size: 20px;
}

.empty-card p {
  margin: 8px 0 0;
  color: var(--wx-muted);
  font-size: 14px;
}

.empty-actions {
  margin-top: 20px;
  display: flex;
  justify-content: center;
  gap: 10px;
}

/* ==================== Conversation Info ==================== */
.conversation-info {
  position: absolute;
  top: 0;
  right: -320px;
  width: 320px;
  height: 100%;
  background: #fff;
  border-left: 1px solid var(--wx-border);
  transition: right 0.24s ease;
  z-index: 16;
  overflow: auto;
}

.conversation-info.show {
  right: 0;
}

.conversation-info > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px;
  border-bottom: 1px solid var(--wx-border);
  background: var(--wx-pink-light);
}

.conversation-info h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.info-block {
  padding: 16px 14px;
  border-bottom: 1px solid var(--wx-border-soft);
}

.profile-block {
  display: flex;
  align-items: center;
  gap: 12px;
}

.info-block h4 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--wx-text);
}

.info-block h5 {
  margin: 0 0 10px;
  color: var(--wx-text);
  font-size: 13px;
  font-weight: 600;
}

.info-type-badge {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--wx-muted);
}

.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 0;
  font-size: 13px;
}

.info-row span:first-child {
  color: var(--wx-muted);
}

.info-row span:last-child {
  color: var(--wx-text);
  font-weight: 500;
}

.text-online {
  color: #16a34a !important;
}

.text-offline {
  color: var(--wx-muted) !important;
}

.info-block label {
  display: grid;
  gap: 4px;
  margin-bottom: 8px;
}

.info-block label span {
  font-size: 12px;
  color: var(--wx-muted);
}

.info-block input {
  height: 34px;
  border-radius: 8px;
  border: 1.5px solid var(--wx-border);
  background: #fff;
  padding: 0 10px;
  color: var(--wx-text);
  font-size: 13px;
  transition: border-color 0.2s;
}

.info-block input:focus {
  outline: none;
  border-color: #e88aab;
}

.info-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
}

.info-actions button {
  height: 32px;
  border-radius: 6px;
  border: 1px solid var(--wx-border);
  background: #fff;
  color: var(--wx-text);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}

.info-actions button:hover {
  border-color: #e88aab;
  background: var(--wx-pink-soft);
  color: var(--wx-pink);
}

/* ==================== Modals ==================== */
.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(45, 31, 36, 0.4);
  backdrop-filter: blur(4px);
  display: grid;
  place-items: center;
  padding: 16px;
  z-index: 30;
}

.modal-card {
  width: min(420px, 94vw);
  border-radius: 16px;
  background: #fff;
  border: 1px solid var(--wx-border);
  padding: 20px;
  box-shadow: 0 20px 60px -10px rgba(180, 60, 100, 0.2);
}

.modal-card > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.modal-card h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.modal-close {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  border: 0;
  background: transparent;
  color: var(--wx-muted);
  cursor: pointer;
  display: grid;
  place-items: center;
  transition: all 0.12s;
}

.modal-close:hover {
  background: var(--wx-pink-soft);
  color: var(--wx-pink);
}

.modal-card label {
  display: grid;
  gap: 4px;
  margin-bottom: 14px;
}

.modal-card label span {
  font-size: 13px;
  color: var(--wx-muted);
  font-weight: 500;
}

.modal-card input,
.modal-card select,
.modal-card textarea {
  width: 100%;
  border: 1.5px solid var(--wx-border);
  border-radius: 8px;
  padding: 9px 12px;
  background: #fff;
  color: var(--wx-text);
  font-size: 14px;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.modal-card input:focus,
.modal-card select:focus,
.modal-card textarea:focus {
  outline: none;
  border-color: #e88aab;
  box-shadow: 0 0 0 3px rgba(212, 80, 122, 0.08);
}

.modal-card input::placeholder,
.modal-card textarea::placeholder {
  color: #c4a3ae;
}

.modal-card textarea {
  resize: vertical;
}

.modal-card footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}

.form-tip {
  margin: 0;
  font-size: 12px;
  color: var(--wx-muted);
}

.mode-switch {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
  margin-bottom: 14px;
}

.mode-switch button {
  height: 34px;
  border-radius: 8px;
  border: 1.5px solid var(--wx-border);
  background: #fff;
  cursor: pointer;
  font-size: 13px;
  color: var(--wx-muted);
  font-weight: 500;
  transition: all 0.15s;
}

.mode-switch button.active {
  border-color: #f0a0bd;
  background: var(--wx-pink-soft);
  color: var(--wx-pink);
  font-weight: 600;
}

.profile-summary {
  border-radius: 10px;
  background: var(--wx-pink-light);
  border: 1px solid var(--wx-border);
  padding: 12px;
  margin-bottom: 4px;
}

/* ==================== Buttons ==================== */
.primary-btn {
  height: 36px;
  border-radius: 8px;
  border: 0;
  background: linear-gradient(135deg, #f472b6, #d4507a);
  color: #fff;
  font-weight: 600;
  font-size: 14px;
  padding: 0 18px;
  cursor: pointer;
  transition: opacity 0.15s;
  box-shadow: 0 2px 8px -2px rgba(212, 80, 122, 0.3);
}

.primary-btn:hover {
  opacity: 0.92;
}

.primary-btn-sm {
  height: 32px;
  border-radius: 6px;
  border: 0;
  background: linear-gradient(135deg, #f472b6, #d4507a);
  color: #fff;
  font-weight: 600;
  font-size: 12px;
  padding: 0 14px;
  cursor: pointer;
  transition: opacity 0.15s;
}

.primary-btn-sm:hover {
  opacity: 0.92;
}

.outline-btn {
  height: 36px;
  border-radius: 8px;
  border: 1.5px solid var(--wx-border);
  background: #fff;
  color: var(--wx-text);
  font-size: 14px;
  font-weight: 500;
  padding: 0 18px;
  cursor: pointer;
  transition: all 0.15s;
}

.outline-btn:hover {
  border-color: #e88aab;
  background: var(--wx-pink-soft);
  color: var(--wx-pink);
}

.danger-text {
  border: 0;
  background: transparent;
  color: var(--wx-danger);
  font-size: 12px;
  cursor: pointer;
}

.danger-text:hover {
  opacity: 0.8;
}

/* ==================== Call ==================== */
.call-mask {
  z-index: 34;
}

.call-card {
  width: min(400px, 92vw);
  border-radius: 20px;
  background: linear-gradient(160deg, #2d1f28, #1a1218);
  color: #fff;
  padding: 32px 24px;
  text-align: center;
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.4);
}

.call-icon {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: rgba(212, 80, 122, 0.2);
  display: inline-grid;
  place-items: center;
  margin-bottom: 16px;
  color: #f472b6;
}

.call-card h3 {
  margin: 0;
  font-size: 18px;
}

.call-card p {
  margin: 8px 0 0;
  color: rgba(255, 255, 255, 0.6);
  font-size: 14px;
}

.call-card footer {
  margin-top: 24px;
  display: flex;
  justify-content: center;
  gap: 16px;
}

.accept-btn,
.danger-btn {
  min-width: 100px;
  height: 40px;
  border: 0;
  border-radius: 20px;
  color: #fff;
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  transition: opacity 0.15s;
}

.accept-btn {
  background: linear-gradient(135deg, #34d399, #16a34a);
  box-shadow: 0 4px 12px -2px rgba(22, 163, 74, 0.4);
}

.danger-btn {
  background: linear-gradient(135deg, #f87171, #dc2626);
  box-shadow: 0 4px 12px -2px rgba(220, 38, 38, 0.4);
}

.accept-btn:hover,
.danger-btn:hover {
  opacity: 0.9;
}

.active-call-panel {
  position: fixed;
  right: 18px;
  bottom: 18px;
  width: min(360px, calc(100vw - 36px));
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: #1a1218;
  color: #fff;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.4);
  z-index: 33;
  overflow: hidden;
}

.active-call-panel > header {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
  padding: 12px 14px;
  background: #2d1f28;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.active-call-panel h4 {
  margin: 0;
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.active-call-panel header span {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
}

.active-call-body {
  position: relative;
  min-height: 200px;
  background: #000;
}

.remote-video {
  width: 100%;
  min-height: 200px;
  object-fit: cover;
  display: block;
}

.local-video {
  position: absolute;
  right: 10px;
  bottom: 10px;
  width: 34%;
  min-width: 92px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-radius: 10px;
  background: #000;
}

.audio-placeholder {
  min-height: 200px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: rgba(255, 255, 255, 0.5);
  font-size: 14px;
}

.active-call-panel footer {
  display: flex;
  justify-content: center;
  padding: 14px;
}

/* ==================== Image Preview ==================== */
.preview-card {
  width: min(860px, 96vw);
  border-radius: 16px;
  background: #fff;
  border: 1px solid var(--wx-border);
  padding: 12px;
  box-shadow: 0 20px 60px -10px rgba(0, 0, 0, 0.2);
}

.preview-card img {
  width: 100%;
  max-height: 72vh;
  object-fit: contain;
  border-radius: 10px;
  background: var(--wx-pink-light);
}

.preview-card footer {
  margin-top: 10px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.preview-card a {
  color: var(--wx-pink);
  text-decoration: none;
  font-weight: 500;
  font-size: 14px;
}

.preview-card a:hover {
  color: var(--wx-pink-strong);
}

/* ==================== Toasts ==================== */
.toast-stack {
  position: fixed;
  right: 16px;
  top: 16px;
  display: grid;
  gap: 8px;
  z-index: 40;
}

.toast {
  min-width: 220px;
  max-width: 340px;
  border-radius: 10px;
  padding: 12px 16px;
  color: #fff;
  font-size: 13px;
  font-weight: 500;
  box-shadow: 0 10px 24px rgba(0, 0, 0, 0.15);
}

.toast.success {
  background: linear-gradient(135deg, #34d399, #16a34a);
}

.toast.error {
  background: linear-gradient(135deg, #f87171, #dc2626);
}

.toast.info {
  background: linear-gradient(135deg, #6b7280, #374151);
}

/* ==================== Global Loading ==================== */
.global-loading {
  position: fixed;
  left: 50%;
  bottom: 20px;
  transform: translateX(-50%);
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border-radius: 999px;
  background: rgba(45, 31, 36, 0.88);
  color: #fff;
  font-size: 13px;
  padding: 8px 18px;
  z-index: 45;
  backdrop-filter: blur(8px);
}

.loading-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* ==================== Avatar ==================== */
.avatar {
  display: inline-grid;
  place-items: center;
  overflow: hidden;
  border-radius: 10px;
  background: #e8d0d8;
  color: #fff;
  font-weight: 700;
  flex-shrink: 0;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-fallback {
  background: linear-gradient(135deg, #fda4af, #e11d48);
}

.avatar-sm {
  width: 34px;
  height: 34px;
  font-size: 13px;
  border-radius: 8px;
}

.avatar-md {
  width: 40px;
  height: 40px;
  font-size: 14px;
  border-radius: 10px;
}

.avatar-lg {
  width: 48px;
  height: 48px;
  font-size: 17px;
  border-radius: 12px;
}

/* ==================== Transitions ==================== */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}
.modal-fade-enter-active .modal-card,
.modal-fade-enter-active .call-card,
.modal-fade-enter-active .preview-card,
.modal-fade-leave-active .modal-card,
.modal-fade-leave-active .call-card,
.modal-fade-leave-active .preview-card {
  transition: transform 0.2s ease, opacity 0.2s ease;
}
.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
.modal-fade-enter-from .modal-card,
.modal-fade-enter-from .call-card,
.modal-fade-enter-from .preview-card {
  transform: scale(0.95);
  opacity: 0;
}
.modal-fade-leave-to .modal-card,
.modal-fade-leave-to .call-card,
.modal-fade-leave-to .preview-card {
  transform: scale(0.95);
  opacity: 0;
}

.menu-fade-enter-active,
.menu-fade-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}
.menu-fade-enter-from,
.menu-fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

.toast-slide-enter-active {
  transition: all 0.3s ease;
}
.toast-slide-leave-active {
  transition: all 0.25s ease;
}
.toast-slide-enter-from {
  opacity: 0;
  transform: translateX(30px);
}
.toast-slide-leave-to {
  opacity: 0;
  transform: translateX(30px);
}

.call-slide-enter-active,
.call-slide-leave-active {
  transition: all 0.25s ease;
}
.call-slide-enter-from,
.call-slide-leave-to {
  opacity: 0;
  transform: translateY(20px);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* ==================== Utilities ==================== */
.hidden-file,
.visually-hidden {
  position: fixed;
  left: -9999px;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
}

/* ==================== Responsive ==================== */
@media (max-width: 1080px) {
  .conversation-info {
    width: 280px;
    right: -280px;
  }
}

@media (max-width: 880px) {
  .wx-app {
    padding: 8px;
  }

  .wx-shell {
    height: calc(100dvh - 16px);
    grid-template-columns: 56px minmax(220px, 38%) 1fr;
  }

  .conversation-info {
    display: none;
  }

  .msg-row {
    max-width: 92%;
  }
}

@media (max-width: 700px) {
  .wx-shell {
    grid-template-columns: 52px 1fr;
    grid-template-rows: 280px 1fr;
  }

  .wx-nav-rail {
    grid-row: 1 / span 2;
  }

  .wx-sidebar {
    grid-column: 2;
    grid-row: 1;
    border-right: 0;
    border-bottom: 1px solid var(--wx-border);
  }

  .wx-main {
    grid-column: 2;
    grid-row: 2;
  }

  .chat-topbar-title h1 {
    font-size: 15px;
  }

  .chat-topbar-actions {
    gap: 4px;
  }

  .message-list {
    padding: 12px;
  }

  .composer {
    padding: 8px 10px 10px;
  }
}
</style>
