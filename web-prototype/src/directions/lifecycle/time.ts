// Calendar helpers. One convention: every date is a YYYY-MM-DD string read as UTC midnight.
import { daysUntil, lineVersions, TODAY, useStore, versionById, type Line } from '../../data';

export type State = ReturnType<typeof useStore>;

const DAY = 86400000;
export const ms = (d: string) => Date.parse(d);
export const iso = (t: number) => new Date(t).toISOString().slice(0, 10);
export const addDays = (d: string, n: number) => iso(ms(d) + n * DAY);
export const daysBetween = (a: string, b: string) => Math.round((ms(b) - ms(a)) / DAY);

export function monthStart(d: string) {
  const t = new Date(ms(d));
  return iso(Date.UTC(t.getUTCFullYear(), t.getUTCMonth(), 1));
}
export function nextMonth(d: string) {
  const t = new Date(ms(d));
  return iso(Date.UTC(t.getUTCFullYear(), t.getUTCMonth() + 1, 1));
}
export function months(from: string, to: string) {
  const out: string[] = [];
  for (let m = monthStart(from); m <= to; m = nextMonth(m)) out.push(m);
  return out;
}

/** "1 年 3 个月" / "5 个月" / "12 天" */
export function span(from: string, to: string) {
  const a = new Date(ms(from));
  const b = new Date(ms(to));
  let m = (b.getUTCFullYear() - a.getUTCFullYear()) * 12 + (b.getUTCMonth() - a.getUTCMonth());
  if (b.getUTCDate() < a.getUTCDate()) m -= 1;
  if (m < 1) return `${Math.max(0, daysBetween(from, to))} 天`;
  const y = Math.floor(m / 12);
  const r = m % 12;
  return y ? `${y} 年${r ? ` ${r} 个月` : ''}` : `${r} 个月`;
}

/** "5 天后到期" / "今天到期" / "已过期 3 天" */
export function dueText(date: string) {
  const d = daysUntil(date);
  return d < 0 ? `已过期 ${-d} 天` : d === 0 ? '今天到期' : `${d} 天后到期`;
}

/** Where a line starts on the calendar: its 源版本's 发布时间, or the first version for 主线. */
export function lineStart(s: State, l: Line) {
  if (l.sourceVersionId) return versionById(l.sourceVersionId)?.releasedAt ?? TODAY;
  return lineVersions(s, l.id)[0]?.releasedAt ?? TODAY;
}

/**
 * Where a line stops being drawn solid.
 * The data has no termination date, so a 终止 line ends at its 计划终止日期 if that is already past,
 * at TODAY if it was terminated ahead of plan, or at its last version if it never had a plan.
 */
export function lineSolidEnd(s: State, l: Line) {
  if (l.status !== '终止') return TODAY;
  if (l.plannedEnd) return l.plannedEnd < TODAY ? l.plannedEnd : TODAY;
  return lineVersions(s, l.id).at(-1)?.releasedAt ?? lineStart(s, l);
}

export const fmtMonthDay = (d: string) => `${Number(d.slice(5, 7))}-${d.slice(8, 10)}`;
