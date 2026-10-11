// PROTOTYPE ONLY: direction contract + the floating bar. Not part of any design being evaluated.
import { useEffect, useSyncExternalStore, type ComponentType } from 'react';
import { actions, useStore, userName, type Role } from './data';
import { setLocale, useT } from './i18n';
import type { Tab } from './router';

export type Direction = {
  Login: ComponentType;
  AppList: ComponentType;
  AppDetail: ComponentType<{ appKey: string; tab: Tab }>;
  /** Global version registry across all applications. */
  Versions: ComponentType;
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

export function PrototypeBar({ names, current }: { names: Record<string, string>; current: string }) {
  const s = useStore();
  const { locale } = useT();
  const keys = Object.keys(names);
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
      style={{ fontFamily: '-apple-system, "PingFang SC", sans-serif', fontSize: 13, whiteSpace: 'nowrap' }}
      className="fixed bottom-4 left-1/2 z-[100] flex -translate-x-1/2 items-center gap-4 bg-[#111] px-4 py-2 text-white"
    >
      <span className="text-white/50">原型</span>
      <span className="flex items-center gap-2">
        <button onClick={() => cycle(-1)} aria-label="上一个方案" className="px-1">←</button>
        <span className="min-w-28 text-center">方案 {names[current]}</span>
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
      <span className="flex gap-1 border-l border-white/20 pl-4">
        <span className="text-white/50">语言</span>
        {(['zh', 'en'] as const).map((l) => (
          <button key={l} onClick={() => setLocale(l)} className={`px-1.5 ${locale === l ? 'font-semibold underline underline-offset-4' : 'text-white/70'}`}>
            {l === 'zh' ? '中文' : 'EN'}
          </button>
        ))}
      </span>
      <span className="text-white/40">{userName(s.me)}</span>
    </div>
  );
}
