// 中文字典。与 ../en/index.ts 一一对应；`pnpm check:i18n` 检查两边 key 完全一致。
import type { MessageKey } from '../en/index.ts';
import applications from './applications.ts';
import audit from './audit.ts';
import common from './common.ts';
import departments from './departments.ts';
import errors from './errors.ts';
import history from './history.ts';
import ldap from './ldap.ts';
import lineage from './lineage.ts';
import login from './login.ts';
import password from './password.ts';
import platforms from './platforms.ts';
import roles from './roles.ts';
import security from './security.ts';
import settings from './settings.ts';
import smtp from './smtp.ts';
import storage from './storage.ts';
import terms from './terms.ts';
import users from './users.ts';
import versions from './versions.ts';

export const sections = { applications, audit, common, departments, errors, history, ldap, lineage, login, password, platforms, roles, security, settings, smtp, storage, terms, users, versions };

export const zh: Record<MessageKey, string> = { ...applications, ...audit, ...common, ...departments, ...errors, ...history, ...ldap, ...lineage, ...login, ...password, ...platforms, ...roles, ...security, ...settings, ...smtp, ...storage, ...terms, ...users, ...versions };
