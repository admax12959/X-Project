// 版本流水 (releaselog): the tool as a kept engineering ledger, newest first.
import '@fontsource/ibm-plex-sans/400.css';
import '@fontsource/ibm-plex-sans/500.css';
import '@fontsource/ibm-plex-sans/600.css';
import '@fontsource/ibm-plex-mono/400.css';
import '@fontsource/ibm-plex-mono/500.css';
import './releaselog.css';
import type { Direction } from '../../prototype';
import { AdminPlatforms, AdminUsers, Login, Settings } from './account';
import { AppDetail } from './detail';
import { AppList } from './home';

export const direction: Direction = { name: '版本流水', Login, AppList, AppDetail, Settings, AdminPlatforms, AdminUsers };
