// Direction B · Catalog: dark narrow sidebar, white panels on a warm grey canvas, filter rail, mainline-spine lineage, releases-style versions.
import type { Direction } from '../../prototype';
import './catalog.css';
import './messages';
import { AppDetail } from './detail';
import { AppList } from './list';
import { AdminPlatforms, AdminUsers, Login, Settings } from './pages';
import { Versions } from './versions';

export const direction: Direction = { Login, AppList, AppDetail, Versions, Settings, AdminPlatforms, AdminUsers };
