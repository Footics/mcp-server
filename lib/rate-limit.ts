import { RATE_LIMIT_PER_MIN } from "@/lib/config";

const WINDOW_MS = 60_000;
const MAX_ENTRIES = 10_000;

type Window = { startedAt: number; count: number };
const windows = new Map<string, Window>();

export function rateLimit(userId: string, now = Date.now()): number {
  if (RATE_LIMIT_PER_MIN <= 0) return 0;
  const w = windows.get(userId);
  if (!w || now - w.startedAt >= WINDOW_MS) {
    if (windows.size >= MAX_ENTRIES) prune(now);
    windows.set(userId, { startedAt: now, count: 1 });
    return 0;
  }
  if (w.count < RATE_LIMIT_PER_MIN) {
    w.count += 1;
    return 0;
  }
  return Math.max(1, Math.ceil((w.startedAt + WINDOW_MS - now) / 1000));
}

function prune(now: number): void {
  for (const [key, w] of windows) {
    if (now - w.startedAt >= WINDOW_MS) windows.delete(key);
  }
  if (windows.size >= MAX_ENTRIES) windows.clear();
}
