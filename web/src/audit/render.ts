// Audit-log rendering: the API sends {action, target, params}; the sentence is built here in the
// current UI language from the message "audit.event.<action>", the action's short name comes from
// "audit.action.<action>". User input inside params (login names, emails, application names) is shown
// verbatim, never translated.
//
// To add an audit action: see "Audit log" in docs/dev.md. One entry in `renderers` below is all the
// frontend code it needs; the filter on the audit page lists these entries.
import type { AuditActor, AuditEntry } from '../api/types.ts';
import { getLocale, hasKey, translate, type MessageKey, type Params } from '../i18n/index.ts';
import { joinNames, userName } from '../history/render.ts';

type Renderer = (e: AuditEntry) => Params;

/** The user the entry is about: the target when it is a user, otherwise whoever params name. */
function targetUser(e: AuditEntry): string {
  return userName(e.target.type === 'user' ? e.target : undefined);
}

function str(v: unknown): string {
  return typeof v === 'string' ? v : '';
}

function kindName(kind: unknown): string {
  return translate(kind === 'ldap' ? 'audit.kind.ldap' : 'audit.kind.email');
}

function roleName(role: unknown): string {
  return translate(role === 'pe' ? 'term.pe' : 'term.owner');
}

/** One entry per backend AuditAction (internal/domain/audit.go): maps the entry to message placeholders. */
const renderers: Record<string, Renderer> = {
  userCreated: (e) => ({ user: targetUser(e), loginEmail: str(e.params.loginEmail) }),
  loginEmailChanged: (e) => ({ user: targetUser(e), from: str(e.params.from), to: str(e.params.to) }),
  identityBound: (e) => ({
    user: targetUser(e),
    kind: kindName(e.params.kind),
    loginName: str(e.params.loginName),
    via: translate(e.params.via === 'self' ? 'audit.via.self' : 'audit.via.admin'),
  }),
  identityUnbound: (e) => ({ user: targetUser(e), kind: kindName(e.params.kind), loginName: str(e.params.loginName) }),
  appMemberAdded: (e) => appMember(e),
  appMemberRemoved: (e) => appMember(e),
  departmentCreated: (e) => ({ name: str(e.params.name), parent: deptName(e.params.parent, 'audit.department.top') }),
  departmentRenamed: (e) => ({ from: str(e.params.from), to: str(e.params.to) }),
  departmentMoved: (e) => ({
    name: str(e.params.name),
    from: deptName(e.params.from, 'audit.department.top'),
    to: deptName(e.params.to, 'audit.department.top'),
  }),
  departmentDeleted: (e) => ({ name: str(e.params.name), parent: deptName(e.params.parent, 'audit.department.top') }),
  userDepartmentChanged: (e) => ({
    user: targetUser(e),
    from: deptName(e.params.from, 'audit.department.none'),
    to: deptName(e.params.to, 'audit.department.none'),
  }),
  secondFactorDisabled: (e) => ({ user: targetUser(e) }),
  recoveryCodesRegenerated: (e) => ({ user: targetUser(e) }),
  secondFactorReset: (e) => ({ user: targetUser(e) }),
  passwordReset: (e) => ({ user: targetUser(e) }),
  secondFactorEnabled: (e) => ({ user: targetUser(e), type: translate('audit.secondFactor.totp') }),
  configChanged: (e) => ({
    section: hasKey(`audit.section.${str(e.params.section)}`) ? translate(`audit.section.${str(e.params.section)}` as MessageKey) : str(e.params.section),
    fields: configFields(e),
  }),
  adminFlagChanged: (e) => ({
    user: targetUser(e),
    change: translate(e.params.to === true ? 'audit.adminFlag.granted' : 'audit.adminFlag.revoked'),
  }),
};

/** A department reference {id, name} from params; null is the top level (or, for a user, no department). */
function deptName(ref: unknown, none: 'audit.department.top' | 'audit.department.none'): string {
  const name = (ref as { name?: unknown } | null)?.name;
  return typeof name === 'string' ? name : translate(none);
}

/** The changed field names of a configuration section, shown with the labels of that section's page. */
function configFields(e: AuditEntry): string {
  const names = Array.isArray(e.params.changes) ? e.params.changes.map(str) : [];
  const label = (f: string) => (hasKey(`${str(e.params.section)}.field.${f}`) ? translate(`${str(e.params.section)}.field.${f}` as MessageKey) : f);
  return joinNames(names.map(label), getLocale());
}

function appMember(e: AuditEntry): Params {
  const app = e.params.application as { key?: string; name?: string } | undefined;
  return {
    user: userName(e.params.user),
    role: roleName(e.params.role),
    application: `${app?.name ?? e.target.id} (${app?.key ?? e.target.id})`,
  };
}

/** Every action the page can render, for the action filter. */
export const auditActions: string[] = Object.keys(renderers);

export function renderAuditEvent(e: AuditEntry): string {
  const key = `audit.event.${e.action}`;
  const render = renderers[e.action];
  if (!render || !hasKey(key)) return translate('audit.event.unknown', { action: e.action });
  return translate(key, render(e));
}

export function auditActionName(action: string): string {
  const key = `audit.action.${action}`;
  return hasKey(key) ? translate(key as MessageKey) : action;
}

export function auditOperatorName(actor: AuditActor): string {
  return actor.kind === 'cli' ? translate('audit.operator.cli') : userName(actor);
}
