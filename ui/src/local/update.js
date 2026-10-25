import { platform } from '../platform/index.js';

const RELEASE_API = 'https://api.github.com/repos/KrustyLeBot/PersonalApp/releases/tags/latest';

// Looks at the rolling "latest" release: its title is "PersonalDashboard build N".
export async function checkForUpdate() {
  const info = await platform.getInfo();
  const res = await platform.httpGet(RELEASE_API, {
    Accept: 'application/vnd.github+json',
    'User-Agent': 'PersonalDashboard',
  });
  if (res.status === 404) return { available: false, current: info.build, latest: info.build };
  if (res.status !== 200) throw new Error(`GitHub a répondu HTTP ${res.status}`);

  const release = JSON.parse(res.body);
  const latest = Number(/build\s+(\d+)/i.exec(release.name || '')?.[1] || 0);
  const asset = (release.assets || []).find((a) => a.name === platform.assetName);
  if (!asset || latest <= info.build) return { available: false, current: info.build, latest };

  let sha256 = '';
  const sidecar = (release.assets || []).find((a) => a.name === `${platform.assetName}.sha256`);
  if (sidecar) {
    const s = await platform.httpGet(sidecar.browser_download_url, { 'User-Agent': 'PersonalDashboard' });
    if (s.status === 200) sha256 = s.body.trim().split(/\s+/)[0].toLowerCase();
  }
  return {
    available: true,
    current: info.build,
    latest,
    asset: { url: asset.browser_download_url, size: asset.size, sha256 },
  };
}

export const downloadUpdate = (asset, onProgress) => platform.downloadUpdate(asset, onProgress);
export const installUpdate = () => platform.installUpdate();
