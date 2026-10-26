import { registerPlugin } from '@capacitor/core';

const Bridge = registerPlugin('AppBridge');

export const android = {
  name: 'android',
  assetName: 'PersonalDashboard.apk',
  canChooseFolder: false,

  getInfo: () => Bridge.getInfo(),
  requestAccess: () => Bridge.requestAccess(),

  async readFile(name) {
    const res = await Bridge.readFile({ name });
    return res.content ?? null;
  },
  writeFile: (name, content) => Bridge.writeFile({ name, content }),
  httpGet: (url, headers = {}) => Bridge.httpGet({ url, headers }),

  async downloadUpdate(asset, onProgress) {
    const handle = await Bridge.addListener('updateProgress', (e) => onProgress(e.done, e.total));
    try {
      await Bridge.downloadUpdate({ url: asset.url, size: asset.size });
    } finally {
      handle.remove();
    }
  },
  installUpdate: () => Bridge.installUpdate(),

  // Proton Drive sync through the Synchrotron app (the PC apps are synced by Proton's own client).
  sync: {
    // handler({state: syncing|done|failed|timeout|skipped, changes, conflicts, reason}); returns a stop function.
    async onOpen(handler) {
      const handle = await Bridge.addListener('syncState', handler);
      await Bridge.syncOnOpen();
      return () => handle.remove();
    },
  },
};
