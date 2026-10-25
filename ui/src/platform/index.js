import { Capacitor } from '@capacitor/core';
import { android } from './android.js';
import { windows } from './windows.js';

export const platform = Capacitor.isNativePlatform() ? android : windows;
