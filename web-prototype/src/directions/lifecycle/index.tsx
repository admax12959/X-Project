// 生命周期 direction: time is the primary axis on every page, on a real calendar scale.
import '@fontsource-variable/archivo/wght.css';
import './lifecycle.css';
import type { Direction } from '../../prototype';
import { AdminPlatforms, AdminUsers } from './Admin';
import { AppDetail } from './AppDetail';
import { AppList } from './AppList';
import { Login } from './Login';
import { Settings } from './Settings';

export const direction: Direction = { name: '生命周期', Login, AppList, AppDetail, Settings, AdminPlatforms, AdminUsers };
