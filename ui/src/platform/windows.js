// The Windows app serves this build from a local server and exposes native
// capabilities as POST /__bridge/<name>. The access token is passed once in the
// page URL so other local processes or web pages cannot call the bridge.
const token = new URLSearchParams(location.search).get('t') || sessionStorage.getItem('bridgeToken') || '';
try { sessionStorage.setItem('bridgeToken', token); } catch { /* storage unavailable */ }

const nativeFetch = window.fetch.bind(window);

async function call(name, body = {}) {
  const res = await nativeFetch(`/__bridge/${name}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Token': token },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error((await res.text()) || `bridge ${name}: HTTP ${res.status}`);
  return res.json();
}

export const windows = {
  name: 'windows',
  assetName: 'PersonalDashboard.exe',
  canChooseFolder: true,

  getInfo: () => call('info'),
  async requestAccess() {},

  async readFile(name) {
    return (await call('read_file', { name })).content;
  },
  writeFile: (name, content) => call('write_file', { name, content }),
  httpGet: (url, headers = {}) => call('http_get', { url, headers }),
  pickFolder: () => call('pick_folder'),

  async downloadUpdate(asset, onProgress) {
    const timer = setInterval(async () => {
      try {
        const p = await call('update_progress');
        onProgress(p.done, p.total);
      } catch { /* the download call reports real errors */ }
    }, 200);
    try {
      await call('download_update', { url: asset.url, size: asset.size, sha256: asset.sha256 });
    } finally {
      clearInterval(timer);
    }
  },
  installUpdate: () => call('install_update'),

  // No sync capability: the PC apps are synced by Proton's own client.
  sync: null,
};
