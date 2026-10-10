// PROTOTYPE ONLY: design-direction registry + the floating bar. Not part of any design being evaluated.
import { useEffect, useSyncExternalStore, type ComponentType } from 'react';
import { actions, useStore, userName, type Role } from './data';

export type Direction = {
  name: string;
  Login: ComponentType;
  AppList: ComponentType;
  AppDetail: ComponentType<{ appKey: string }>;
  Settings: ComponentType;
  AdminPlatforms: ComponentType;
  AdminUsers: ComponentType;
};

// Direction choice lives in localStorage: navigate() drops search params on path changes.
const KEY = 'proto:direction';
const listeners = new Set<() => void>();
export function useDirectionKey(keys: string[]) {
  const v = useSyncExternalStore(
    (cb) => (listeners.add(cb), () => listeners.delete(cb)),
    () => localStorage.getItem(KEY),
  );
  return v && keys.includes(v) ? v : keys[0];
}
export function setDirectionKey(k: string) {
  localStorage.setItem(KEY, k);
  listeners.forEach((l) => l());
}

const ROLES: { key: Role; label: string }[] = [
  { key: 'admin', label: '管理员' },
  { key: 'owner', label: 'Owner' },
  { key: 'pe', label: 'PE' },
];

export function PrototypeBar({ directions, current }: { directions: Record<string, Direction>; current: string }) {
  const s = useStore();
  const keys = Object.keys(directions);
  const cycle = (d: number) => setDirectionKey(keys[(keys.indexOf(current) + d + keys.length) % keys.length]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (t.closest('input, textarea, select, [contenteditable]') || document.querySelector('[role=dialog]')) return;
      if (e.altKey && e.key === 'ArrowLeft') cycle(-1);
      if (e.altKey && e.key === 'ArrowRight') cycle(1);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  if (!import.meta.env.DEV) return null;
  return (
    <div
      data-prototype-bar
      style={{ fontFamily: '-apple-system, "PingFang SC", sans-serif', fontSize: 13 }}
      className="fixed bottom-4 left-1/2 z-[100] flex -translate-x-1/2 items-center gap-4 bg-[#111] px-4 py-2 text-white"
    >
      <span className="text-white/50">原型</span>
      <span className="flex items-center gap-2">
        <button onClick={() => cycle(-1)} aria-label="上一个方案" className="px-1">←</button>
        <span className="min-w-36 text-center">方案 {current} · {directions[current].name}</span>
        <button onClick={() => cycle(1)} aria-label="下一个方案" className="px-1">→</button>
      </span>
      <span className="flex gap-1 border-l border-white/20 pl-4">
        <span className="text-white/50">视角</span>
        {ROLES.map((r) => (
          <button key={r.key} onClick={() => actions.setRole(r.key)} className={`px-1.5 ${s.role === r.key ? 'font-semibold underline underline-offset-4' : 'text-white/70'}`}>
            {r.label}
          </button>
        ))}
      </span>
      <span className="text-white/40">{userName(s.me)}</span>
    </div>
  );
}
