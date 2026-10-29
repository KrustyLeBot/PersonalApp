// Minimal browser-like globals so the modules that import the platform layer can load under node:test.
globalThis.window = globalThis;
globalThis.location = { search: '' };
globalThis.sessionStorage = { getItem: () => null, setItem: () => {} };
export const writes = [];
// The data folder as seen through the Windows bridge; tests may change it like another device would.
export const disk = {};
globalThis.fetch = async (url, init) => {
  const body = JSON.parse(init.body);
  let out = {};
  if (String(url).endsWith('write_file')) {
    writes.push(body);
    disk[body.name] = body.content;
  } else if (String(url).endsWith('read_file')) {
    out = { content: disk[body.name] ?? null };
  } else if (String(url).endsWith('/info')) {
    out = { hasAccess: true, build: 0 };
  }
  return { ok: true, json: async () => out, text: async () => '' };
};
