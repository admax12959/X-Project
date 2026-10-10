/*
 * 方案「工作台」(workbench) — defended choices
 *
 * Who it is for: Owners who look after many maintenance lines all day. The 分支谱系 is the product,
 * so it is always on screen when an 应用 is open, and everything else is arranged around it.
 *
 * Layout: three panes. Left = a tree (应用 → 主线 / 维护线 nested by lineage, then 个人设置 and, for
 * 管理员, 管理后台). Center = canvas. Right = inspector for whatever is selected (应用 → 线 → 版本,
 * with a breadcrumb to step back out). Selection lives in the URL (?line= / ?ver=), so the nav tree
 * can deep-link to a line. Keyboard: j/k (↑/↓) lines, h/l (←/→) versions, Esc steps out.
 *
 * Typeface: the OS's own UI face — PingFang SC on macOS, Microsoft YaHei UI on Windows, with
 * -apple-system / Segoe UI ahead of them for Latin and digits. A Chinese internal tool read all day
 * needs the CJK face the OS hints best, and digits from the system Latin face are tabular, which the
 * version/date columns rely on. Hierarchy comes from weight (400/600/700) and a real scale
 * (11.5 / 12.5 / 13.5 / 15 / 20 / 28px), not from grey. Mono only for 应用标识, branches, tags,
 * commits and repo paths — things you would paste into git.
 *
 * Colour: near-black ink on white, three ink steps that all pass contrast, hairlines in two greys.
 * Selection is shown by inversion (black row, white text), never by a hue. The one accent is
 * 朱 vermilion (#b5321b), the colour of a reviewer's annotation: it means "a person needs to act" —
 * a 计划终止 inside 30 days, a validation error, an irreversible action. Nothing decorative uses it,
 * so when it appears on screen it is always worth reading.
 *
 * Status without traffic lights: 活跃/冻结/终止 are drawn as stroke styles shared by the graph, the
 * nav tree and tables — heavy single rail / hollow double rail / thin grey rail ending in a bar —
 * and always paired with the word.
 *
 * Density: 13.5px body, 28px buttons, 7px table cell padding, 60px graph rows. Tight inside a group,
 * 28–40px between groups.
 *
 * Focal element per page:
 * - 登录: the form; the left half says in one sentence what the tool records.
 * - 应用列表: 「30 天内到期的维护线」, first, with the day count set large in 朱. The full index
 *   below is a dense sortable table (default sort: soonest 计划终止).
 * - 应用详情: the 分支谱系 rail chart (graph.tsx explains the drawing rules). Inspector on the right
 *   holds 应用信息 / 部署相关信息 / 变更历史 when nothing is selected; line or version detail otherwise.
 * - 个人设置: 登录身份 — the only thing a user actually changes here.
 * - 代码平台: which 应用 depend on each platform (so nobody pulls a token that is in use).
 * - 用户: status and 「唯一 Owner」, the fact that decides whether a user can be disabled.
 */
import type { Direction } from '../../prototype';
import { AdminPlatforms, AdminUsers, AppDetail, AppList, Login, Settings } from './pages';
import './workbench.css';

export const direction: Direction = { name: '工作台', Login, AppList, AppDetail, Settings, AdminPlatforms, AdminUsers };
