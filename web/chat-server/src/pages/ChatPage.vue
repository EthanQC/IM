<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { storeToRefs } from "pinia";
import { useRouter } from "vue-router";
import type { NormalizedMessage, UserBrief } from "../types/im";
import { formatMessageTime } from "../utils/message";
import { toErrorMessage } from "../services/response";
import { useAuthStore } from "../stores/auth";
import { useIMStore } from "../stores/im";

const router = useRouter();
const auth = useAuthStore();
const im = useIMStore();

const {
  initializing,
  loading,
  wsConnected,
  wsError,
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
  isCalling,
} = storeToRefs(im);

const { profile, userId, userName } = storeToRefs(auth);

const sidebarMode = ref<"conversation" | "contact">("conversation");
const messageInput = ref("");
const callTargetUserId = ref<number | null>(null);
const localVideoEl = ref<HTMLVideoElement | null>(null);
const remoteVideoEl = ref<HTMLVideoElement | null>(null);
const remoteAudioEl = ref<HTMLAudioElement | null>(null);
const fileInputEl = ref<HTMLInputElement | null>(null);

const createConversationForm = reactive({
  type: 1 as 1 | 2,
  title: "",
  memberIDsText: "",
});

const renameForm = reactive({
  enabled: false,
  title: "",
});

const applyForm = reactive({
  targetUserId: 0,
  remark: "",
});

const handleForm = reactive({
  targetUserId: 0,
  accept: true,
});

const profileForm = reactive({
  displayName: "",
  avatarURL: "",
});

const toasts = ref<Array<{ id: string; type: "success" | "error"; text: string }>>([]);

const callPhaseText = computed(() => {
  switch (callPhase.value) {
    case "incoming":
      return "有来电";
    case "outgoing":
      return "呼叫中";
    case "connecting":
      return "连接建立中";
    case "connected":
      return "通话中";
    default:
      return "空闲";
  }
});

const activeReadTip = computed(() => {
  if (!activeConversationId.value) {
    return "";
  }
  return im.formatReadTip(activeConversationId.value);
});

const statusText = computed(() => {
  if (wsConnected.value) {
    return "实时通道已连接";
  }
  return "实时通道离线";
});

function pushToast(type: "success" | "error", text: string): void {
  const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  toasts.value = [...toasts.value, { id, type, text }];

  window.setTimeout(() => {
    toasts.value = toasts.value.filter((item) => item.id !== id);
  }, 2800);
}

function getUserLabel(user: UserBrief | null | undefined): string {
  if (!user) {
    return "未知用户";
  }
  return user.display_name || user.username || `用户 ${user.id}`;
}

function senderName(senderID: number): string {
  if (senderID === userId.value) {
    return "我";
  }
  const hit = contacts.value.find((item) => item.id === senderID);
  if (!hit) {
    return `用户 ${senderID}`;
  }
  return getUserLabel(hit);
}

function mediaURL(message: NormalizedMessage): string {
  if (!message.media?.object_key) {
    return "";
  }
  return im.toFileURL(message.media.object_key);
}

function hasUsableURL(url: string): boolean {
  return /^https?:\/\//.test(url);
}

function showMessagePreview(message: NormalizedMessage): string {
  if (message.revoked) {
    return "消息已撤回";
  }

  switch (message.kind) {
    case "text":
      return message.text || "文本消息";
    case "image":
      return "[图片]";
    case "audio":
      return "[语音]";
    case "video":
      return "[视频]";
    case "file":
      return "[文件]";
    case "call":
      return "[通话消息]";
    default:
      return "[消息]";
  }
}

async function selectConversation(conversationId: number): Promise<void> {
  await im.openConversation(conversationId);
  renameForm.enabled = false;
  renameForm.title = activeConversation.value?.title || "";
}

async function refreshCurrentConversation(): Promise<void> {
  if (!activeConversationId.value) {
    return;
  }
  await im.loadHistory(activeConversationId.value);
  pushToast("success", "历史消息已刷新");
}

async function submitTextMessage(): Promise<void> {
  const text = messageInput.value.trim();
  if (!text) {
    return;
  }

  try {
    await im.sendTextMessage(text);
    messageInput.value = "";
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

function triggerFilePicker(): void {
  fileInputEl.value?.click();
}

async function onFileSelected(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) {
    return;
  }

  try {
    await im.sendFileMessage(file);
    pushToast("success", "文件发送成功");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  } finally {
    input.value = "";
  }
}

async function revokeMessage(messageID: number): Promise<void> {
  try {
    await im.revokeMessage(messageID);
    pushToast("success", "消息已撤回");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function submitCreateConversation(): Promise<void> {
  try {
    await im.createConversation({
      type: createConversationForm.type,
      title: createConversationForm.title.trim() || undefined,
      memberIDsText: createConversationForm.memberIDsText,
    });

    createConversationForm.title = "";
    createConversationForm.memberIDsText = "";
    pushToast("success", "会话创建成功");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

function enableRenameConversation(): void {
  if (!activeConversation.value) {
    return;
  }
  renameForm.enabled = true;
  renameForm.title = activeConversation.value.title;
}

async function submitRenameConversation(): Promise<void> {
  if (!activeConversationId.value || !renameForm.title.trim()) {
    return;
  }

  try {
    await im.renameConversation(activeConversationId.value, renameForm.title.trim());
    renameForm.enabled = false;
    pushToast("success", "会话名称已更新");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function refreshConversationDetail(): Promise<void> {
  if (!activeConversationId.value) {
    return;
  }

  await im.refreshConversationDetail(activeConversationId.value);
}

async function submitApplyContact(): Promise<void> {
  if (!applyForm.targetUserId) {
    pushToast("error", "请填写目标用户 ID");
    return;
  }

  try {
    await im.applyContact(applyForm.targetUserId, applyForm.remark.trim());
    applyForm.remark = "";
    pushToast("success", "好友申请已发送");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function submitHandleContact(): Promise<void> {
  if (!handleForm.targetUserId) {
    pushToast("error", "请填写目标用户 ID");
    return;
  }

  try {
    await im.handleContact(handleForm.targetUserId, handleForm.accept);
    pushToast("success", handleForm.accept ? "已同意好友申请" : "已拒绝好友申请");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function removeContact(user: UserBrief): Promise<void> {
  const confirmed = window.confirm(`确认删除联系人 ${getUserLabel(user)} 吗？`);
  if (!confirmed) {
    return;
  }

  try {
    await im.deleteContact(user.id);
    pushToast("success", "联系人已删除");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function startChatFromContact(user: UserBrief): Promise<void> {
  try {
    await im.startSingleChatWithUser(user.id);
    sidebarMode.value = "conversation";
    callTargetUserId.value = user.id;
    pushToast("success", `已打开与 ${getUserLabel(user)} 的会话`);
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function refreshEverything(): Promise<void> {
  await Promise.all([im.loadConversations(), im.loadContacts()]);
  await im.refreshPresence();
  if (activeConversationId.value) {
    await im.loadHistory(activeConversationId.value);
  }
  pushToast("success", "数据已刷新");
}

async function saveProfile(): Promise<void> {
  try {
    await auth.updateProfile({
      display_name: profileForm.displayName.trim() || undefined,
      avatar_url: profileForm.avatarURL.trim() || undefined,
    });
    pushToast("success", "资料已更新");
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
}

async function markCurrentRead(): Promise<void> {
  if (!activeConversationId.value) {
    return;
  }
  await im.markConversationRead(activeConversationId.value);
}

async function startCall(type: "audio" | "video"): Promise<void> {
  if (!callTargetUserId.value) {
    pushToast("error", "请输入对方用户 ID");
    return;
  }

  try {
    await im.initiateCall(callTargetUserId.value, type);
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
  auth.logout();
  im.resetState();
  await router.replace({ name: "login" });
}

watch(
  profile,
  (nextProfile) => {
    profileForm.displayName = nextProfile?.user?.display_name || "";
    profileForm.avatarURL = nextProfile?.user?.avatar_url || "";
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

onMounted(async () => {
  try {
    await im.initialize();
  } catch (error) {
    pushToast("error", toErrorMessage(error));
  }
});

onBeforeUnmount(() => {
  im.disconnectRealtime();
});
</script>

<template>
  <main class="chat-shell">
    <header class="topbar">
      <div class="topbar-left">
        <span class="brand">IM</span>
        <div>
          <p class="topbar-title">即时通讯控制台</p>
          <p class="topbar-subtitle">{{ statusText }}</p>
        </div>
      </div>

      <div class="topbar-right">
        <span class="badge" :class="{ online: wsConnected }">
          {{ wsConnected ? "在线" : "离线" }}
        </span>
        <button class="ghost-btn" @click="refreshEverything">刷新</button>
        <button class="ghost-btn" @click="logout">退出登录</button>
      </div>
    </header>

    <section class="layout">
      <aside class="sidebar">
        <section class="profile-card">
          <img v-if="profile?.user?.avatar_url" :src="profile.user.avatar_url" alt="avatar" />
          <div class="avatar-fallback" v-else>{{ (userName || "?" ).slice(0, 1).toUpperCase() }}</div>
          <div>
            <p class="profile-name">{{ userName || "未命名用户" }}</p>
            <p class="profile-id">ID: {{ userId || "-" }}</p>
          </div>
        </section>

        <section class="switcher">
          <button :class="{ active: sidebarMode === 'conversation' }" @click="sidebarMode = 'conversation'">
            会话
          </button>
          <button :class="{ active: sidebarMode === 'contact' }" @click="sidebarMode = 'contact'">联系人</button>
        </section>

        <section v-if="sidebarMode === 'conversation'" class="list-wrap">
          <button class="small-btn" @click="im.loadConversations">刷新会话列表</button>
          <ul v-if="conversations.length" class="list">
            <li
              v-for="conversation in conversations"
              :key="conversation.id"
              :class="{ active: activeConversationId === conversation.id }"
              @click="selectConversation(conversation.id)"
            >
              <div class="list-item-top">
                <p class="list-title">{{ conversation.title || `会话 #${conversation.id}` }}</p>
                <span v-if="(unreadMap[conversation.id] || 0) > 0" class="unread">
                  {{ unreadMap[conversation.id] }}
                </span>
              </div>
              <p class="list-desc">{{ im.formatConversationSubtitle(conversation.id) }}</p>
            </li>
          </ul>
          <p v-else class="empty">暂无会话</p>
        </section>

        <section v-else class="list-wrap">
          <button class="small-btn" @click="im.refreshPresence">刷新在线状态</button>
          <ul v-if="contacts.length" class="list">
            <li v-for="contact in contacts" :key="contact.id">
              <div class="list-item-top">
                <p class="list-title">{{ getUserLabel(contact) }}</p>
                <span class="presence" :class="{ online: presenceMap[contact.id]?.online }">
                  {{ presenceMap[contact.id]?.online ? "在线" : "离线" }}
                </span>
              </div>
              <p class="list-desc">ID: {{ contact.id }}</p>
              <div class="inline-actions">
                <button class="small-btn" @click.stop="startChatFromContact(contact)">聊天</button>
                <button class="small-btn danger" @click.stop="removeContact(contact)">删除</button>
              </div>
            </li>
          </ul>
          <p v-else class="empty">暂无联系人</p>
        </section>
      </aside>

      <section class="chat-main">
        <template v-if="activeConversation">
          <header class="conversation-head">
            <div>
              <h2>{{ activeConversation.title || `会话 #${activeConversation.id}` }}</h2>
              <p>ID: {{ activeConversation.id }} | 类型: {{ activeConversation.type === 2 ? "群聊" : "单聊" }}</p>
            </div>
            <div class="head-actions">
              <button class="small-btn" @click="refreshConversationDetail">刷新详情</button>
              <button class="small-btn" @click="refreshCurrentConversation">刷新消息</button>
              <button class="small-btn" @click="markCurrentRead">标记已读</button>
              <button class="small-btn" @click="enableRenameConversation">改名</button>
            </div>
          </header>

          <form v-if="renameForm.enabled" class="rename-form" @submit.prevent="submitRenameConversation">
            <input v-model="renameForm.title" placeholder="输入新会话标题" />
            <button class="small-btn" type="submit">保存</button>
            <button class="small-btn" type="button" @click="renameForm.enabled = false">取消</button>
          </form>

          <p v-if="activeReadTip" class="read-tip">{{ activeReadTip }}</p>

          <section class="message-list" :class="{ loading: loading }">
            <article
              v-for="message in activeMessages"
              :key="message.id || `${message.conversationId}-${message.seq}`"
              class="message-item"
              :class="{ self: message.senderId === userId }"
            >
              <header class="message-meta">
                <span>{{ senderName(message.senderId) }}</span>
                <span>#{{ message.seq }}</span>
                <span>{{ formatMessageTime(message.createdAtUnix) }}</span>
                <button
                  v-if="message.senderId === userId && !message.revoked"
                  class="mini-link"
                  @click="revokeMessage(message.id)"
                >
                  撤回
                </button>
              </header>

              <div class="message-bubble">
                <template v-if="message.revoked">
                  <p class="muted">这条消息已撤回</p>
                </template>

                <template v-else-if="message.kind === 'text'">
                  <p>{{ message.text }}</p>
                </template>

                <template v-else-if="message.kind === 'image'">
                  <template v-if="hasUsableURL(mediaURL(message))">
                    <img :src="mediaURL(message)" alt="image message" class="message-image" />
                  </template>
                  <template v-else>
                    <p>图片对象: {{ message.media?.object_key }}</p>
                  </template>
                </template>

                <template v-else-if="message.kind === 'audio'">
                  <template v-if="hasUsableURL(mediaURL(message))">
                    <audio controls :src="mediaURL(message)"></audio>
                  </template>
                  <template v-else>
                    <p>语音对象: {{ message.media?.object_key }}</p>
                  </template>
                </template>

                <template v-else-if="message.kind === 'video'">
                  <template v-if="hasUsableURL(mediaURL(message))">
                    <video controls class="message-video" :src="mediaURL(message)"></video>
                  </template>
                  <template v-else>
                    <p>视频对象: {{ message.media?.object_key }}</p>
                  </template>
                </template>

                <template v-else-if="message.kind === 'file'">
                  <template v-if="hasUsableURL(mediaURL(message))">
                    <a :href="mediaURL(message)" target="_blank" rel="noreferrer">{{ message.media?.filename || "下载文件" }}</a>
                  </template>
                  <template v-else>
                    <p>文件对象: {{ message.media?.object_key }}</p>
                  </template>
                </template>

                <template v-else>
                  <p>{{ showMessagePreview(message) }}</p>
                </template>
              </div>
            </article>

            <p v-if="!activeMessages.length" class="empty">暂无消息，开始发送第一条吧。</p>
          </section>

          <footer class="composer">
            <textarea
              v-model="messageInput"
              placeholder="输入消息，按 Enter 发送，Shift+Enter 换行"
              @keydown.enter.exact.prevent="submitTextMessage"
            />
            <div class="composer-actions">
              <input ref="fileInputEl" type="file" class="hidden-file" @change="onFileSelected" />
              <button class="small-btn" :disabled="uploading" @click="triggerFilePicker">
                {{ uploading ? "上传中..." : "上传文件" }}
              </button>
              <button class="send-btn" :disabled="sending" @click="submitTextMessage">
                {{ sending ? "发送中..." : "发送" }}
              </button>
            </div>
          </footer>
        </template>

        <section v-else class="empty-main">
          <h2>选择一个会话开始聊天</h2>
          <p>你可以在右侧创建新会话，或从联系人中发起单聊。</p>
        </section>
      </section>

      <aside class="tool-panel">
        <section class="tool-card">
          <h3>创建会话</h3>
          <label>
            会话类型
            <select v-model.number="createConversationForm.type">
              <option :value="1">单聊</option>
              <option :value="2">群聊</option>
            </select>
          </label>
          <label>
            会话标题（群聊建议填写）
            <input v-model="createConversationForm.title" placeholder="例如：项目讨论组" />
          </label>
          <label>
            成员 ID（逗号分隔）
            <input v-model="createConversationForm.memberIDsText" placeholder="2,3,4" />
          </label>
          <button class="small-btn" @click="submitCreateConversation">创建</button>
        </section>

        <section class="tool-card">
          <h3>联系人操作</h3>
          <label>
            申请添加用户 ID
            <input v-model.number="applyForm.targetUserId" type="number" placeholder="例如 2" />
          </label>
          <label>
            申请备注
            <input v-model="applyForm.remark" placeholder="你好，我是..." />
          </label>
          <button class="small-btn" @click="submitApplyContact">发送申请</button>

          <hr />

          <label>
            处理申请用户 ID
            <input v-model.number="handleForm.targetUserId" type="number" placeholder="例如 2" />
          </label>
          <label>
            处理结果
            <select v-model="handleForm.accept">
              <option :value="true">同意</option>
              <option :value="false">拒绝</option>
            </select>
          </label>
          <button class="small-btn" @click="submitHandleContact">提交处理</button>
        </section>

        <section class="tool-card">
          <h3>个人资料</h3>
          <label>
            显示名
            <input v-model="profileForm.displayName" placeholder="新的显示名" />
          </label>
          <label>
            头像 URL
            <input v-model="profileForm.avatarURL" placeholder="https://..." />
          </label>
          <button class="small-btn" @click="saveProfile">保存资料</button>
        </section>

        <section class="tool-card">
          <h3>通话控制（WebRTC）</h3>
          <label>
            对方用户 ID
            <input v-model.number="callTargetUserId" type="number" placeholder="例如 2" />
          </label>
          <div class="inline-actions">
            <button class="small-btn" :disabled="isCalling" @click="startCall('audio')">发起语音</button>
            <button class="small-btn" :disabled="isCalling" @click="startCall('video')">发起视频</button>
          </div>

          <p class="tool-note">当前状态: {{ callPhaseText }}</p>
          <p v-if="activeCall">当前通话 ID: {{ activeCall.callId }}</p>
          <p v-if="incomingCall" class="tool-note">来自用户 {{ incomingCall.fromUserId }} 的来电</p>

          <div v-if="incomingCall" class="inline-actions">
            <button class="small-btn" @click="acceptCall">接听</button>
            <button class="small-btn danger" @click="rejectCall">拒绝</button>
          </div>

          <button v-if="isCalling && !incomingCall" class="small-btn danger" @click="hangupCall">挂断</button>

          <p v-if="callError" class="error-text">{{ callError }}</p>
        </section>

        <section class="tool-card state-card">
          <h3>连接状态</h3>
          <p>{{ statusText }}</p>
          <p v-if="wsError" class="error-text">{{ wsError }}</p>
          <p v-if="errorMessage" class="error-text">{{ errorMessage }}</p>
        </section>
      </aside>
    </section>

    <section v-if="isCalling" class="call-overlay">
      <header>
        <h3>实时通话 - {{ callPhaseText }}</h3>
        <p v-if="activeCall">对方 ID: {{ activeCall.peerUserId }}</p>
      </header>

      <div class="call-media">
        <div>
          <p>远端画面</p>
          <video ref="remoteVideoEl" autoplay playsinline controls></video>
        </div>
        <div>
          <p>本地预览</p>
          <video ref="localVideoEl" autoplay muted playsinline controls></video>
        </div>
      </div>
      <audio ref="remoteAudioEl" autoplay></audio>

      <footer>
        <button class="small-btn danger" @click="hangupCall">结束通话</button>
      </footer>
    </section>

    <section class="toast-container">
      <article v-for="toast in toasts" :key="toast.id" class="toast" :class="toast.type">{{ toast.text }}</article>
    </section>
  </main>
</template>

<style scoped>
.chat-shell {
  min-height: 100dvh;
  padding: 16px 18px 20px;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
  padding: 12px 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--surface);
  backdrop-filter: blur(10px);
}

.topbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.brand {
  display: inline-grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  background: linear-gradient(130deg, #f6a4c1, #f185b0);
  color: #fff;
  font-weight: 700;
}

.topbar-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
}

.topbar-subtitle {
  margin: 2px 0 0;
  color: var(--text-soft);
  font-size: 12px;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.layout {
  display: grid;
  grid-template-columns: minmax(240px, 300px) minmax(0, 1fr) minmax(280px, 350px);
  gap: 12px;
}

.sidebar,
.chat-main,
.tool-panel {
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  backdrop-filter: blur(10px);
  box-shadow: var(--shadow);
}

.sidebar,
.tool-panel {
  display: grid;
  align-content: start;
  gap: 12px;
  padding: 12px;
  max-height: calc(100dvh - 110px);
  overflow: auto;
}

.chat-main {
  display: grid;
  grid-template-rows: auto auto auto minmax(0, 1fr) auto;
  min-height: calc(100dvh - 110px);
}

.profile-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: #fff;
}

.profile-card img,
.avatar-fallback {
  width: 42px;
  height: 42px;
  border-radius: 50%;
  object-fit: cover;
  border: 1px solid var(--border);
}

.avatar-fallback {
  display: grid;
  place-items: center;
  background: #ffe5f0;
  color: #9d486a;
  font-weight: 700;
}

.profile-name {
  margin: 0;
  font-weight: 700;
}

.profile-id {
  margin: 2px 0 0;
  color: var(--text-soft);
  font-size: 12px;
}

.switcher {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.switcher button {
  height: 36px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: #fff;
  color: var(--text-soft);
  cursor: pointer;
}

.switcher button.active {
  background: #ffe5f0;
  color: #8d3f60;
  border-color: #f6bfd5;
}

.list-wrap {
  display: grid;
  gap: 8px;
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 8px;
}

.list li {
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: #fff;
  padding: 10px;
  cursor: pointer;
}

.list li.active {
  border-color: #f6bfd5;
  background: #fff1f7;
}

.list-item-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.list-title {
  margin: 0;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.list-desc {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--text-soft);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.unread {
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 10px;
  background: #ee6e9f;
  color: #fff;
  font-size: 12px;
  display: inline-grid;
  place-items: center;
}

.presence {
  font-size: 12px;
  color: #866776;
}

.presence.online {
  color: #1f9f5f;
}

.inline-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}

.small-btn,
.send-btn,
.ghost-btn {
  height: 32px;
  border-radius: 10px;
  border: 1px solid var(--border);
  padding: 0 12px;
  background: #fff;
  color: var(--text);
  cursor: pointer;
  font-size: 13px;
}

.small-btn:hover,
.ghost-btn:hover {
  background: #fff4f9;
}

.small-btn.danger {
  color: #a3365f;
  border-color: #f1b6cb;
}

.send-btn {
  min-width: 96px;
  border: none;
  color: #fff;
  background: linear-gradient(120deg, #f7a8c6, #f18ab2);
}

.send-btn:disabled,
.small-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.badge {
  padding: 4px 10px;
  border-radius: 999px;
  border: 1px solid var(--border);
  font-size: 12px;
  color: var(--text-soft);
  background: #fff;
}

.badge.online {
  background: #f2fff7;
  color: #198b52;
  border-color: #a8e0be;
}

.conversation-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
  padding: 14px 14px 10px;
  border-bottom: 1px solid var(--border);
}

.conversation-head h2 {
  margin: 0;
  font-size: 20px;
}

.conversation-head p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-soft);
}

.head-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.rename-form {
  padding: 8px 14px;
  display: flex;
  gap: 8px;
  border-bottom: 1px solid var(--border);
}

.rename-form input {
  flex: 1;
}

.read-tip {
  margin: 0;
  padding: 8px 14px;
  font-size: 12px;
  color: var(--text-soft);
  border-bottom: 1px dashed var(--border);
}

.message-list {
  padding: 14px;
  overflow: auto;
  display: grid;
  align-content: start;
  gap: 10px;
}

.message-item {
  max-width: min(80%, 600px);
}

.message-item.self {
  margin-left: auto;
}

.message-meta {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
  font-size: 11px;
  color: var(--text-soft);
  margin-bottom: 4px;
}

.message-bubble {
  border: 1px solid var(--border);
  border-radius: 12px;
  background: #fff;
  padding: 10px;
  color: var(--text);
  word-break: break-word;
}

.message-item.self .message-bubble {
  background: #fff2f8;
  border-color: #f2bfd4;
}

.message-bubble p {
  margin: 0;
}

.message-image,
.message-video {
  max-width: min(320px, 100%);
  border-radius: 10px;
  border: 1px solid var(--border);
}

.muted {
  color: var(--text-soft);
  font-style: italic;
}

.mini-link {
  border: none;
  background: transparent;
  color: #cf4f7d;
  cursor: pointer;
  font-size: 11px;
  padding: 0;
}

.composer {
  padding: 10px 14px 14px;
  border-top: 1px solid var(--border);
  display: grid;
  gap: 10px;
}

textarea,
input,
select {
  width: 100%;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: #fff;
  color: var(--text);
  padding: 10px 12px;
}

textarea {
  min-height: 96px;
  resize: vertical;
}

textarea:focus,
input:focus,
select:focus {
  outline: 2px solid #ffd0e1;
  outline-offset: 1px;
}

.composer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.hidden-file {
  display: none;
}

.empty,
.empty-main {
  margin: 0;
  text-align: center;
  color: var(--text-soft);
}

.empty-main {
  display: grid;
  place-items: center;
  gap: 8px;
}

.empty-main h2 {
  margin: 0;
}

.tool-card {
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 10px;
  background: #fff;
  display: grid;
  gap: 8px;
}

.tool-card h3 {
  margin: 0;
  font-size: 15px;
}

.tool-card hr {
  width: 100%;
  border: none;
  border-top: 1px dashed var(--border);
  margin: 4px 0;
}

.tool-note {
  margin: 0;
  color: var(--text-soft);
  font-size: 13px;
}

.error-text {
  margin: 0;
  color: #b74169;
  font-size: 12px;
}

.call-overlay {
  position: fixed;
  right: 18px;
  bottom: 18px;
  width: min(460px, calc(100vw - 36px));
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: #fff;
  box-shadow: var(--shadow);
  padding: 12px;
  z-index: 20;
  display: grid;
  gap: 10px;
}

.call-overlay h3 {
  margin: 0;
}

.call-overlay p {
  margin: 3px 0 0;
  color: var(--text-soft);
}

.call-media {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.call-media p {
  margin: 0 0 6px;
  font-size: 12px;
}

.call-media video {
  width: 100%;
  aspect-ratio: 16 / 9;
  background: #151217;
  border-radius: 10px;
}

.toast-container {
  position: fixed;
  left: 50%;
  top: 14px;
  transform: translateX(-50%);
  display: grid;
  gap: 8px;
  z-index: 30;
}

.toast {
  padding: 10px 14px;
  border-radius: 999px;
  background: #fff;
  border: 1px solid var(--border);
  color: var(--text);
  box-shadow: 0 8px 25px -18px rgba(0, 0, 0, 0.3);
}

.toast.success {
  border-color: #bde8ce;
  color: #1b8d55;
}

.toast.error {
  border-color: #f0bfd0;
  color: #b13f67;
}

@media (max-width: 1280px) {
  .layout {
    grid-template-columns: minmax(220px, 280px) minmax(0, 1fr);
  }

  .tool-panel {
    grid-column: 1 / -1;
    max-height: none;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  }
}

@media (max-width: 860px) {
  .chat-shell {
    padding: 10px;
  }

  .topbar {
    flex-direction: column;
    align-items: stretch;
  }

  .layout {
    grid-template-columns: 1fr;
  }

  .sidebar,
  .tool-panel,
  .chat-main {
    max-height: none;
    min-height: auto;
  }

  .message-item {
    max-width: 100%;
  }

  .conversation-head {
    flex-direction: column;
    align-items: flex-start;
  }

  .head-actions {
    justify-content: flex-start;
  }

  .call-media {
    grid-template-columns: 1fr;
  }
}
</style>
