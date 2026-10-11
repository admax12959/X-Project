// English dictionary. One file per feature area; add a new area here AND in ../zh/index.ts.
import applications from './applications.ts';
import common from './common.ts';
import errors from './errors.ts';
import login from './login.ts';
import platforms from './platforms.ts';
import terms from './terms.ts';

export const sections = { applications, common, errors, login, platforms, terms };

export const en = { ...applications, ...common, ...errors, ...login, ...platforms, ...terms };

export type MessageKey = keyof typeof en;
