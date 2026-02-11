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
    pushToast("success", "会话与联系人已刷新");
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
  const confirmed = window.confirm(`确认删除联系人“${userLabel(contact)}”？`);
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
      <nav class="wx-nav-rail">
        <button class="nav-avatar" @click="openProfileDialog" title="我的资料">
          <img v-if="hasUsableURL(currentUserAvatar)" :src="currentUserAvatar" alt="我的头像" />
          <span v-else>{{ avatarLetter(currentUserName) }}</span>
        </button>

        <button :class="{ active: leftMode === 'chats' }" @click="leftMode = 'chats'" title="聊天">💬</button>
        <button :class="{ active: leftMode === 'contacts' }" @click="leftMode = 'contacts'" title="通讯录">👥</button>
        <button @click="openNewChatDialog" title="新建聊天">＋</button>

        <div class="nav-spacer"></div>
        <button @click="refreshEverything()" title="刷新">⟳</button>
        <button @click="logout" title="退出登录">↩</button>
      </nav>

      <aside class="wx-sidebar">
        <div class="me-card">
          <div v-if="hasUsableURL(currentUserAvatar)" class="avatar avatar-lg">
            <img :src="currentUserAvatar" alt="我的头像" />
          </div>
          <div v-else class="avatar avatar-lg avatar-fallback">{{ avatarLetter(currentUserName) }}</div>

          <div class="me-meta">
            <h2>{{ currentUserName }}</h2>
            <p>{{ myAccount }}</p>
          </div>

          <span class="status-chip" :class="{ online: wsConnected }">{{ connectionLabel }}</span>
        </div>

        <div class="sidebar-search">
          <input v-model="searchKeyword" type="search" placeholder="搜索会话/联系人" />
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
              <div class="avatar avatar-md avatar-fallback">{{ avatarLetter(getConversationTitle(conversation)) }}</div>

              <div class="chat-item-main">
                <div class="chat-item-head">
                  <span class="chat-title">{{ getConversationTitle(conversation) }}</span>
                  <span class="chat-time">{{ conversationTime(conversation) }}</span>
                </div>
                <p class="chat-subtitle">{{ conversationPreview(conversation) }}</p>
              </div>

              <span v-if="unreadMap[conversation.id]" class="unread-badge">{{ unreadMap[conversation.id] }}</span>
            </button>

            <div v-if="!filteredConversations.length" class="empty-list">暂无会话</div>
          </template>

          <template v-else>
            <article v-for="contact in filteredContacts" :key="contact.id" class="contact-item">
              <div class="avatar avatar-md" :class="{ 'avatar-fallback': !hasUsableURL(contact.avatar_url) }">
                <img v-if="hasUsableURL(contact.avatar_url)" :src="contact.avatar_url" :alt="userLabel(contact)" />
                <span v-else>{{ avatarLetter(userLabel(contact)) }}</span>
              </div>

              <div class="contact-main" @click="openChatFromContact(contact)">
                <div class="contact-name-row">
                  <h3>{{ userLabel(contact) }}</h3>
                  <span class="contact-online" :class="{ online: isOnline(contact.id) }">
                    {{ isOnline(contact.id) ? "在线" : "离线" }}
                  </span>
                </div>
                <p>{{ `@${contact.username}` }}</p>
              </div>

              <button class="danger-text" @click="removeContact(contact)">删除</button>
            </article>

            <div v-if="!filteredContacts.length" class="empty-list">暂无联系人</div>
          </template>
        </div>

        <div class="sidebar-actions">
          <button @click="openNewChatDialog">新建聊天</button>
          <button @click="openAddFriendDialog">添加好友</button>
        </div>
      </aside>

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
            <button class="icon-btn" :disabled="!activeConversation" @click="openConversationInfo" title="会话信息">ℹ</button>

            <div ref="actionMenuEl" class="menu-wrap">
              <button class="icon-btn" @click.stop="toggleActionMenu" title="更多">⋯</button>

              <div v-if="showActionMenu" class="action-menu" @click.stop>
                <button @click="openNewChatDialog">发起聊天</button>
                <button @click="openAddFriendDialog">添加好友</button>
                <button @click="openHandleApplyDialog">处理好友申请</button>
                <button @click="openProfileDialog">编辑资料</button>
                <button @click="refreshEverything()">刷新</button>
                <button class="danger-text" @click="logout">退出登录</button>
              </div>
            </div>
          </div>
        </header>

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
                <div v-else class="avatar avatar-sm avatar-fallback">{{ avatarLetter(senderName(message.senderId)) }}</div>
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
                      <span class="file-icon">📎</span>
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
                  <button v-if="canRevoke(message)" class="link-btn" @click="revokeMessage(message.id)">撤回</button>
                </footer>
              </div>
            </article>
          </div>

          <div v-if="dragOver" class="drop-mask">松开鼠标，发送文件</div>

          <footer class="composer">
            <div class="composer-tools">
              <div class="composer-tool-left">
                <button class="ghost-btn" :disabled="uploading" @click="openFilePicker">发送文件</button>
                <button
                  v-if="activeConversation && activeConversation.type === 1"
                  class="ghost-btn"
                  :disabled="Boolean(activeCall)"
                  @click="startCall('audio')"
                >
                  语音通话
                </button>
                <button
                  v-if="activeConversation && activeConversation.type === 1"
                  class="ghost-btn"
                  :disabled="Boolean(activeCall)"
                  @click="startCall('video')"
                >
                  视频通话
                </button>
              </div>
              <span>支持 Ctrl/Cmd + V 直接发送截图</span>
            </div>

            <textarea
              v-model="messageInput"
              class="composer-input"
              rows="4"
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
                发送
              </button>
            </div>
          </footer>
        </div>

        <section v-else class="chat-empty">
          <div class="empty-card">
            <h2>欢迎使用 IM</h2>
            <p>从左侧选择一个会话，或点击右上角“⋯”创建新的聊天。</p>
            <div class="empty-actions">
              <button @click="openNewChatDialog">发起聊天</button>
              <button @click="openAddFriendDialog">添加好友</button>
            </div>
          </div>
        </section>
      </section>

      <aside class="conversation-info" :class="{ show: showConversationInfo }">
        <header>
          <h3>会话信息</h3>
          <button class="icon-btn" @click="closeConversationInfo">✕</button>
        </header>

        <template v-if="activeConversation">
          <section class="info-block profile-block">
            <div class="avatar avatar-lg avatar-fallback">{{ avatarLetter(activeConversationTitle) }}</div>
            <div>
              <h4>{{ activeConversationTitle }}</h4>
              <p>{{ activeConversation.type === 2 ? "群聊" : "单聊" }}</p>
            </div>
          </section>

          <section v-if="activePeer" class="info-block">
            <h5>联系人信息</h5>
            <p>昵称：{{ userLabel(activePeer) }}</p>
            <p>账号：{{ `@${activePeer.username}` }}</p>
            <p>IM 号：{{ formatIMCode(activePeer.id) }}</p>
            <p>状态：{{ isOnline(activePeer.id) ? "在线" : "离线" }}</p>
          </section>

          <section v-if="activeConversation.type === 2" class="info-block">
            <h5>群聊设置</h5>
            <label>
              <span>群聊名称</span>
              <input v-model="conversationTitleForm.title" type="text" placeholder="输入群聊名称" />
            </label>
            <button @click="saveConversationTitle">保存名称</button>
          </section>

          <section class="info-block">
            <h5>快捷操作</h5>
            <button @click="openAddFriendDialog">添加好友</button>
            <button @click="openHandleApplyDialog">处理好友申请</button>
            <button @click="openProfileDialog">编辑我的资料</button>
            <button @click="refreshEverything()">刷新数据</button>
          </section>

          <section class="info-block">
            <h5>连接状态</h5>
            <p>{{ wsConnected ? "实时连接正常" : "正在重连" }}</p>
            <p>通话状态：{{ callPhaseText }}</p>
          </section>
        </template>
      </aside>
    </section>

    <input ref="fileInputEl" type="file" multiple class="hidden-file" @change="onFileSelected" />
    <audio ref="remoteAudioEl" autoplay playsinline class="visually-hidden"></audio>

    <div v-if="showAddFriendModal" class="modal-mask" @click.self="showAddFriendModal = false">
      <section class="modal-card">
        <header>
          <h3>添加好友</h3>
          <button class="icon-btn" @click="showAddFriendModal = false">✕</button>
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
          <button class="ghost-btn" @click="showAddFriendModal = false">取消</button>
          <button @click="submitAddFriend">发送申请</button>
        </footer>
      </section>
    </div>

    <div v-if="showHandleApplyModal" class="modal-mask" @click.self="showHandleApplyModal = false">
      <section class="modal-card">
        <header>
          <h3>处理好友申请</h3>
          <button class="icon-btn" @click="showHandleApplyModal = false">✕</button>
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

        <p class="form-tip">如果你看不到申请来源，请让对方把账号或 IM 号发给你再处理。</p>

        <footer>
          <button class="ghost-btn" @click="showHandleApplyModal = false">取消</button>
          <button @click="submitHandleApply">提交处理</button>
        </footer>
      </section>
    </div>

    <div v-if="showNewChatModal" class="modal-mask" @click.self="showNewChatModal = false">
      <section class="modal-card">
        <header>
          <h3>发起聊天</h3>
          <button class="icon-btn" @click="showNewChatModal = false">✕</button>
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
            <span>成员 IM 号 / 用户 ID（逗号或换行分隔）</span>
            <textarea
              v-model="newChatForm.groupMembers"
              rows="4"
              placeholder="IM-00002A, 29\nIM-00002B"
            />
          </label>
        </template>

        <footer>
          <button class="ghost-btn" @click="showNewChatModal = false">取消</button>
          <button @click="submitNewChat">创建</button>
        </footer>
      </section>
    </div>

    <div v-if="showProfileModal" class="modal-mask" @click.self="showProfileModal = false">
      <section class="modal-card">
        <header>
          <h3>我的资料</h3>
          <button class="icon-btn" @click="showProfileModal = false">✕</button>
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
          <p>{{ `账号：${myAccount}` }}</p>
          <p>{{ `IM 号：${myIMCode}` }}</p>
        </section>

        <footer>
          <button class="ghost-btn" @click="showProfileModal = false">取消</button>
          <button @click="saveProfile">保存</button>
        </footer>
      </section>
    </div>

    <div v-if="incomingCall" class="modal-mask call-mask" @click.self="rejectCall">
      <section class="call-card">
        <h3>{{ `${userLabelById(incomingCall.fromUserId)} 邀请你${incomingCall.callType === 'video' ? '视频通话' : '语音通话'}` }}</h3>
        <p>请在 30 秒内处理来电</p>
        <footer>
          <button class="danger-btn" @click="rejectCall">拒绝</button>
          <button class="accept-btn" @click="acceptCall">接听</button>
        </footer>
      </section>
    </div>

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
        <div v-else class="audio-placeholder">语音通话进行中</div>

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

    <div v-if="imagePreview" class="modal-mask" @click.self="closeImagePreview">
      <section class="preview-card">
        <img :src="imagePreview.url" :alt="imagePreview.name" />
        <footer>
          <a :href="imagePreview.url" target="_blank" rel="noreferrer">在新窗口打开</a>
          <button @click="closeImagePreview">关闭</button>
        </footer>
      </section>
    </div>

    <div class="toast-stack">
      <article v-for="toast in toasts" :key="toast.id" class="toast" :class="toast.type">
        {{ toast.text }}
      </article>
    </div>

    <div v-if="initializing || loading" class="global-loading">同步中...</div>
  </main>
</template>

<style scoped>
.wx-app {
  --wx-bg: #f3f3f3;
  --wx-surface: #ffffff;
  --wx-side: #ededed;
  --wx-border: #d9d9d9;
  --wx-border-soft: #e9e9e9;
  --wx-text: #111111;
  --wx-muted: #7e7e7e;
  --wx-green: #07c160;
  --wx-green-strong: #06ad56;
  --wx-danger: #eb4d4b;
  min-height: 100dvh;
  width: 100%;
  padding: 14px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.8), rgba(255, 255, 255, 0.5)),
    var(--wx-bg);
}

.wx-shell {
  position: relative;
  display: grid;
  grid-template-columns: 64px minmax(260px, 320px) 1fr;
  height: calc(100dvh - 28px);
  border: 1px solid var(--wx-border);
  border-radius: 10px;
  overflow: hidden;
  background: var(--wx-surface);
}

.wx-nav-rail {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 10px 6px;
  border-right: 1px solid #d5d5d5;
  background: #2f3136;
}

.wx-nav-rail button {
  width: 42px;
  height: 42px;
  border-radius: 10px;
  border: 0;
  background: transparent;
  color: #d7d9dd;
  cursor: pointer;
  font-size: 18px;
  display: grid;
  place-items: center;
}

.wx-nav-rail button:hover {
  background: #40444b;
}

.wx-nav-rail button.active {
  background: #1f9957;
  color: #fff;
}

.wx-nav-rail .nav-avatar {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: #4b5058;
  color: #fff;
  overflow: hidden;
  margin-bottom: 4px;
}

.wx-nav-rail .nav-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.nav-spacer {
  flex: 1;
}

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
  padding: 14px;
  border-bottom: 1px solid var(--wx-border);
}

.me-meta {
  min-width: 0;
  flex: 1;
}

.me-meta h2 {
  margin: 0;
  font-size: 17px;
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
  border-radius: 999px;
  border: 1px solid #cccccc;
  color: var(--wx-muted);
  font-size: 11px;
  padding: 2px 8px;
  line-height: 18px;
  background: #ffffff;
}

.status-chip.online {
  color: var(--wx-green-strong);
  border-color: #9ed9b8;
  background: #effaf3;
}

.sidebar-search {
  padding: 10px 14px;
}

.sidebar-search input {
  width: 100%;
  height: 34px;
  border-radius: 6px;
  border: 1px solid var(--wx-border-soft);
  background: #ffffff;
  padding: 0 10px;
  font-size: 14px;
}

.sidebar-search input:focus {
  outline: none;
  border-color: #9ed9b8;
  box-shadow: 0 0 0 3px rgba(7, 193, 96, 0.1);
}

.sidebar-tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  padding: 0 14px 10px;
}

.sidebar-tabs button {
  height: 34px;
  border-radius: 6px;
  border: 1px solid var(--wx-border);
  background: #f7f7f7;
  font-size: 14px;
  color: #575757;
  cursor: pointer;
}

.sidebar-tabs button.active {
  border-color: #b9d9c5;
  background: #effaf3;
  color: #1a8f4e;
  font-weight: 600;
}

.sidebar-list {
  overflow: auto;
  padding: 2px 0 8px;
}

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
}

.chat-item:hover {
  background: #f5f5f5;
}

.chat-item.active {
  background: #e8e8e8;
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
  color: #222;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chat-time {
  font-size: 11px;
  color: #9a9a9a;
  white-space: nowrap;
}

.chat-subtitle {
  margin: 4px 0 0;
  color: #8b8b8b;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.unread-badge {
  min-width: 20px;
  height: 20px;
  border-radius: 10px;
  background: #fa5151;
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  padding: 0 6px;
}

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
  color: #222;
  font-weight: 600;
}

.contact-main p {
  margin: 3px 0 0;
  font-size: 12px;
  color: #8a8a8a;
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
  font-size: 11px;
  color: #9b9b9b;
}

.contact-online.online {
  color: #0d9b50;
}

.sidebar-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  padding: 12px 14px;
  border-top: 1px solid var(--wx-border);
}

.sidebar-actions button {
  height: 36px;
  border-radius: 6px;
  border: 1px solid #c6c6c6;
  background: #fff;
  color: #333;
  cursor: pointer;
}

.sidebar-actions button:hover {
  background: #f6f6f6;
}

.empty-list {
  padding: 24px 16px;
  text-align: center;
  color: #8f8f8f;
  font-size: 13px;
}

.wx-main {
  display: grid;
  grid-template-rows: auto 1fr;
  background: #f5f5f5;
}

.chat-topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  padding: 14px 18px;
  border-bottom: 1px solid var(--wx-border);
  background: #f7f7f7;
}

.chat-topbar-title {
  min-width: 0;
}

.chat-topbar-title h1 {
  margin: 0;
  font-size: 18px;
  color: var(--wx-text);
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chat-topbar-title p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--wx-muted);
}

.chat-topbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.icon-btn {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  border: 1px solid #d2d2d2;
  background: #fff;
  color: #666;
  cursor: pointer;
}

.icon-btn:hover:not(:disabled) {
  background: #f4f4f4;
}

.icon-btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.menu-wrap {
  position: relative;
}

.action-menu {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  min-width: 190px;
  border: 1px solid var(--wx-border);
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.12);
  padding: 8px;
  z-index: 24;
}

.action-menu button {
  width: 100%;
  text-align: left;
  border: 0;
  border-radius: 6px;
  background: transparent;
  padding: 8px 10px;
  color: #333;
  cursor: pointer;
}

.action-menu button:hover {
  background: #f5f5f5;
}

.chat-body {
  position: relative;
  display: grid;
  grid-template-rows: 1fr auto;
  min-height: 0;
}

.message-list {
  overflow: auto;
  padding: 18px 24px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.msg-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px;
  align-items: flex-start;
  max-width: min(84%, 760px);
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
  margin: 0 0 4px;
  font-size: 12px;
  color: #888;
}

.msg-bubble {
  display: inline-block;
  max-width: 100%;
  border-radius: 6px;
  padding: 10px 12px;
  background: #fff;
  border: 1px solid #ededed;
  color: #1f1f1f;
}

.msg-bubble.mine {
  background: #95ec69;
  border-color: #86de5a;
}

.msg-bubble.media {
  padding: 4px;
}

.msg-bubble.revoked {
  background: #f5f5f5;
  border-color: #ececec;
}

.msg-text {
  margin: 0;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.msg-image {
  display: block;
  max-width: min(320px, 60vw);
  border-radius: 6px;
  cursor: zoom-in;
}

.msg-video {
  width: min(340px, 65vw);
  border-radius: 6px;
  background: #000;
}

.msg-file {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #2f6f4a;
  text-decoration: none;
  word-break: break-all;
}

.file-icon {
  font-size: 18px;
}

.msg-call {
  margin: 0;
  color: #444;
}

.revoked-text {
  color: #888;
  font-size: 13px;
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
  color: #9b9b9b;
}

.link-btn {
  border: 0;
  background: transparent;
  color: #6c6c6c;
  font-size: 11px;
  cursor: pointer;
  padding: 0;
}

.link-btn:hover {
  color: #3f3f3f;
}

.drop-mask {
  position: absolute;
  inset: 0;
  background: rgba(7, 193, 96, 0.1);
  border: 2px dashed rgba(7, 193, 96, 0.35);
  color: #11753f;
  font-weight: 600;
  display: grid;
  place-items: center;
  pointer-events: none;
  z-index: 5;
}

.composer {
  border-top: 1px solid var(--wx-border);
  background: #fff;
  padding: 10px 14px 12px;
}

.composer-tools {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
  color: #8d8d8d;
  font-size: 12px;
}

.composer-tool-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.composer-input {
  width: 100%;
  resize: none;
  min-height: 100px;
  border: 0;
  border-radius: 6px;
  padding: 8px;
  background: #fff;
  font-size: 14px;
  line-height: 1.6;
}

.composer-input:focus {
  outline: none;
}

.composer-actions {
  margin-top: 8px;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
}

.composer-state {
  font-size: 12px;
  color: #888;
}

.send-btn {
  min-width: 92px;
  height: 34px;
  border: 0;
  border-radius: 6px;
  background: var(--wx-green);
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.send-btn:hover:not(:disabled) {
  background: var(--wx-green-strong);
}

.send-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.chat-empty {
  display: grid;
  place-items: center;
  padding: 18px;
}

.empty-card {
  width: min(460px, 92%);
  border: 1px solid #e7e7e7;
  border-radius: 10px;
  background: #fff;
  padding: 30px;
  text-align: center;
}

.empty-card h2 {
  margin: 0;
  color: #222;
}

.empty-card p {
  margin: 10px 0 0;
  color: #828282;
}

.empty-actions {
  margin-top: 16px;
  display: flex;
  justify-content: center;
  gap: 10px;
}

.empty-actions button {
  min-width: 100px;
  height: 34px;
  border-radius: 6px;
  border: 1px solid #d3d3d3;
  background: #fff;
  color: #333;
  cursor: pointer;
}

.conversation-info {
  position: absolute;
  top: 0;
  right: -320px;
  width: 320px;
  height: 100%;
  background: #fafafa;
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
  background: #f3f3f3;
}

.conversation-info h3 {
  margin: 0;
  font-size: 16px;
}

.info-block {
  padding: 14px;
  border-bottom: 1px solid var(--wx-border-soft);
}

.profile-block {
  display: flex;
  align-items: center;
  gap: 10px;
}

.info-block h4,
.info-block h5 {
  margin: 0 0 8px;
  color: #1f1f1f;
}

.info-block p {
  margin: 6px 0;
  font-size: 13px;
  color: #6e6e6e;
}

.info-block label {
  display: grid;
  gap: 6px;
  margin-bottom: 8px;
}

.info-block label span {
  font-size: 12px;
  color: #777;
}

.info-block input,
.info-block button {
  height: 34px;
  border-radius: 6px;
  border: 1px solid #d8d8d8;
  background: #fff;
  padding: 0 10px;
  color: #333;
}

.info-block button {
  cursor: pointer;
  margin-top: 6px;
}

.error-text {
  color: #d13d3b;
}

.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.42);
  display: grid;
  place-items: center;
  padding: 16px;
  z-index: 30;
}

.modal-card {
  width: min(440px, 94vw);
  border-radius: 10px;
  background: #fff;
  border: 1px solid #d6d6d6;
  padding: 16px;
}

.modal-card > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.modal-card h3 {
  margin: 0;
  font-size: 18px;
}

.modal-card label {
  display: grid;
  gap: 6px;
  margin-bottom: 12px;
}

.modal-card label span {
  font-size: 13px;
  color: #666;
}

.modal-card input,
.modal-card select,
.modal-card textarea {
  width: 100%;
  border: 1px solid #d8d8d8;
  border-radius: 6px;
  padding: 8px 10px;
  background: #fff;
  color: #222;
}

.modal-card textarea {
  resize: vertical;
}

.modal-card footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}

.modal-card footer button,
.modal-card button,
.ghost-btn {
  height: 34px;
  border-radius: 6px;
  border: 1px solid #cfcfcf;
  background: #fff;
  color: #333;
  padding: 0 14px;
  cursor: pointer;
}

.modal-card footer button:last-child {
  background: var(--wx-green);
  border-color: var(--wx-green-strong);
  color: #fff;
}

.modal-card footer button:last-child:hover {
  background: var(--wx-green-strong);
}

.ghost-btn {
  background: #fff;
}

.ghost-btn:hover {
  background: #f5f5f5;
}

.mode-switch {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-bottom: 12px;
}

.mode-switch button {
  height: 34px;
  border-radius: 6px;
  border: 1px solid #d8d8d8;
  background: #f6f6f6;
  cursor: pointer;
}

.mode-switch button.active {
  border-color: #b2d8bf;
  background: #effaf3;
  color: #1a8f4e;
}

.form-tip {
  margin: 0;
  font-size: 12px;
  color: #8a8a8a;
}

.profile-summary {
  border-radius: 6px;
  background: #f7f7f7;
  border: 1px solid #ededed;
  padding: 10px;
}

.profile-summary p {
  margin: 0;
  font-size: 13px;
  color: #666;
}

.profile-summary p + p {
  margin-top: 6px;
}

.call-mask {
  z-index: 34;
}

.call-card {
  width: min(420px, 92vw);
  border-radius: 12px;
  background: #1f1f1f;
  color: #fff;
  padding: 24px;
  text-align: center;
}

.call-card h3 {
  margin: 0;
  font-size: 20px;
}

.call-card p {
  margin: 10px 0 0;
  color: #d3d3d3;
}

.call-card footer {
  margin-top: 16px;
  display: flex;
  justify-content: center;
  gap: 10px;
}

.accept-btn,
.danger-btn {
  min-width: 98px;
  height: 36px;
  border: 0;
  border-radius: 18px;
  color: #fff;
  cursor: pointer;
}

.accept-btn {
  background: var(--wx-green);
}

.danger-btn {
  background: var(--wx-danger);
}

.active-call-panel {
  position: fixed;
  right: 18px;
  bottom: 18px;
  width: min(360px, calc(100vw - 36px));
  border-radius: 12px;
  border: 1px solid #2a2a2a;
  background: #111;
  color: #fff;
  box-shadow: 0 18px 45px rgba(0, 0, 0, 0.35);
  z-index: 33;
  overflow: hidden;
}

.active-call-panel > header {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
  padding: 10px 12px;
  background: #1a1a1a;
  border-bottom: 1px solid #252525;
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
  color: #cfcfcf;
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
  border: 1px solid rgba(255, 255, 255, 0.4);
  border-radius: 8px;
  background: #000;
}

.audio-placeholder {
  min-height: 200px;
  display: grid;
  place-items: center;
  color: #ddd;
  font-size: 15px;
}

.active-call-panel footer {
  display: flex;
  justify-content: center;
  padding: 12px;
}

.preview-card {
  width: min(860px, 96vw);
  border-radius: 10px;
  background: #fff;
  border: 1px solid #d8d8d8;
  padding: 10px;
}

.preview-card img {
  width: 100%;
  max-height: 72vh;
  object-fit: contain;
  border-radius: 8px;
  background: #f4f4f4;
}

.preview-card footer {
  margin-top: 8px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.preview-card a {
  color: #1b8f4e;
  text-decoration: none;
}

.preview-card button {
  height: 32px;
  border: 1px solid #d0d0d0;
  border-radius: 6px;
  background: #fff;
  padding: 0 12px;
  cursor: pointer;
}

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
  border-radius: 8px;
  padding: 10px 12px;
  color: #fff;
  font-size: 13px;
  box-shadow: 0 10px 22px rgba(0, 0, 0, 0.16);
}

.toast.success {
  background: #1d9653;
}

.toast.error {
  background: #d54240;
}

.toast.info {
  background: #4c4c4c;
}

.global-loading {
  position: fixed;
  left: 50%;
  bottom: 16px;
  transform: translateX(-50%);
  border-radius: 999px;
  background: rgba(24, 24, 24, 0.84);
  color: #fff;
  font-size: 12px;
  padding: 6px 12px;
  z-index: 45;
}

.avatar {
  display: inline-grid;
  place-items: center;
  overflow: hidden;
  border-radius: 6px;
  background: #d8d8d8;
  color: #4d4d4d;
  font-weight: 700;
}

.avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.avatar-fallback {
  background: linear-gradient(145deg, #e8e8e8, #d9d9d9);
}

.avatar-sm {
  width: 34px;
  height: 34px;
  font-size: 13px;
}

.avatar-md {
  width: 40px;
  height: 40px;
  font-size: 14px;
}

.avatar-lg {
  width: 52px;
  height: 52px;
  font-size: 18px;
}

.danger-text {
  border: 0;
  background: transparent;
  color: #d54240;
  font-size: 12px;
  cursor: pointer;
}

.hidden-file,
.visually-hidden {
  position: fixed;
  left: -9999px;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
}

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
    max-width: 96%;
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
    font-size: 16px;
  }

  .chat-topbar-actions {
    gap: 6px;
  }

  .message-list {
    padding: 12px;
  }

  .composer {
    padding: 8px 10px 10px;
  }
}
</style>
