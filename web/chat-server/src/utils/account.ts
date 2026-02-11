import type { UserBrief } from "../types/im";

const IM_CODE_PREFIX = "IM";

export function formatIMCode(userId: number): string {
  if (!Number.isFinite(userId) || userId <= 0) {
    return "";
  }
  return `${IM_CODE_PREFIX}-${Math.floor(userId).toString(36).toUpperCase().padStart(6, "0")}`;
}

export function parseIMCode(input: string): number | null {
  const raw = input.trim();
  if (!raw) {
    return null;
  }

  const compact = raw.replace(/[\s-]+/g, "").toUpperCase();
  if (/^\d+$/.test(compact)) {
    const value = Number(compact);
    return Number.isFinite(value) && value > 0 ? Math.floor(value) : null;
  }

  if (!compact.startsWith(IM_CODE_PREFIX)) {
    return null;
  }

  const payload = compact.slice(IM_CODE_PREFIX.length);
  if (!payload || !/^[0-9A-Z]+$/.test(payload)) {
    return null;
  }

  const parsed = Number.parseInt(payload, 36);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return null;
  }
  return parsed;
}

export function resolveUserFromIdentifier(
  input: string,
  contacts: UserBrief[],
  selfUser?: UserBrief | null,
): { userId: number | null; matchedBy: "im_code" | "username" | "display_name" | "none" } {
  const byCode = parseIMCode(input);
  if (byCode) {
    return { userId: byCode, matchedBy: "im_code" };
  }

  const normalized = input.trim().replace(/^@+/, "").toLowerCase();
  if (!normalized) {
    return { userId: null, matchedBy: "none" };
  }

  const pool = [...contacts, ...(selfUser ? [selfUser] : [])];

  const byUsername = pool.find((item) => item.username?.toLowerCase() === normalized);
  if (byUsername) {
    return { userId: Number(byUsername.id), matchedBy: "username" };
  }

  const byDisplayName = pool.find((item) => item.display_name?.toLowerCase() === normalized);
  if (byDisplayName) {
    return { userId: Number(byDisplayName.id), matchedBy: "display_name" };
  }

  return { userId: null, matchedBy: "none" };
}
