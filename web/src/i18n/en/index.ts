// English dictionary. One file per feature area; add a new area here AND in ../zh/index.ts.
import applications from './applications.ts';
import audit from './audit.ts';
import common from './common.ts';
import errors from './errors.ts';
import history from './history.ts';
import ldap from './ldap.ts';
import lineage from './lineage.ts';
import login from './login.ts';
import password from './password.ts';
import platforms from './platforms.ts';
import roles from './roles.ts';
import settings from './settings.ts';
import terms from './terms.ts';
import users from './users.ts';
import versions from './versions.ts';

export const sections = { applications, audit, common, errors, history, ldap, lineage, login, password, platforms, roles, settings, terms, users, versions };

export const en = { ...applications, ...audit, ...common, ...errors, ...history, ...ldap, ...lineage, ...login, ...password, ...platforms, ...roles, ...settings, ...terms, ...users, ...versions };

export type MessageKey = keyof typeof en;
