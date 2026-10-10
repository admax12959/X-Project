// 现状: the first layout prototype, kept only as a baseline to compare against. Not an option.
import type { Direction } from '../../prototype';
import { AdminPlatforms, AdminUsers } from './pages/Admin';
import { AppDetail } from './pages/AppDetail';
import { AppList } from './pages/AppList';
import { Login } from './pages/Login';
import { Settings } from './pages/Settings';

export const baseline: Direction = { name: '现状（对照）', Login, AppList, AppDetail, Settings, AdminPlatforms, AdminUsers };
