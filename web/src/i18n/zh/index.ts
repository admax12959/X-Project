// 中文字典。与 ../en/index.ts 一一对应；`pnpm check:i18n` 检查两边 key 完全一致。
import type { MessageKey } from '../en/index.ts';
import applications from './applications.ts';
import common from './common.ts';
import errors from './errors.ts';
import login from './login.ts';
import platforms from './platforms.ts';
import settings from './settings.ts';
import terms from './terms.ts';
import users from './users.ts';

export const sections = { applications, common, errors, login, platforms, settings, terms, users };

export const zh: Record<MessageKey, string> = { ...applications, ...common, ...errors, ...login, ...platforms, ...settings, ...terms, ...users };
