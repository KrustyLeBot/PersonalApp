// Minimal browser-like globals so the modules that import the platform layer can load under node:test.
globalThis.window = globalThis;
globalThis.location = { search: '' };
globalThis.sessionStorage = { getItem: () => null, setItem: () => {} };
export const writes = [];
globalThis.fetch = async (url, init) => {
  const body = JSON.parse(init.body);
  if (String(url).endsWith('write_file')) writes.push(body);
  return { ok: true, json: async () => ({}), text: async () => '' };
};
