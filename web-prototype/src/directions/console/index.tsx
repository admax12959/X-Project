import type { Direction } from '../../prototype';
import './console.css';
import './messages';
import { AdminPlatforms, AdminUsers, Login, Settings } from './account';
import { AppDetail } from './detail';
import { AppList } from './list';
import { Versions } from './versions';

export const direction: Direction = { Login, AppList, AppDetail, Versions, Settings, AdminPlatforms, AdminUsers };
