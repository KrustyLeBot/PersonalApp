// "Be up to date when I open": Synchrotron (the Proton Drive sync app) syncs the data folder, the app is
// read-only meanwhile and re-reads the files when the sync brought changes. Android only.
import { platform } from '../platform/index.js';
import { setReadOnly } from './syncGuard.js';

const HINT_KEY = 'syncHintDismissed';

function hintDismissed() {
  try { return localStorage.getItem(HINT_KEY) === '1'; } catch { return false; }
}

export function dismissSyncHint() {
  try { localStorage.setItem(HINT_KEY, '1'); } catch { /* storage unavailable */ }
}

// onState receives {syncing, offer, notice}; reload re-reads the data files. Returns a stop function.
export async function startSyncOnOpen({ onState, reload }) {
  if (!platform.sync) return () => {};
  return platform.sync.onOpen(async (e) => {
    switch (e.state) {
      case 'syncing':
        setReadOnly(true);
        onState({ syncing: true, offer: false, notice: '' });
        break;
      case 'done': {
        try {
          if (e.changes > 0) await reload();
        } finally {
          setReadOnly(false);
        }
        const notice = e.conflicts
          ? 'Conflits de synchro : des fichiers .conflict sont dans le dossier'
          : e.changes > 0 ? `Données synchronisées (${e.changes} changement${e.changes > 1 ? 's' : ''})` : '';
        onState({ syncing: false, offer: false, notice });
        break;
      }
      case 'failed':
        setReadOnly(false);
        onState({ syncing: false, offer: false, notice: 'Échec de la synchro Proton Drive' });
        break;
      case 'timeout':
        setReadOnly(false);
        onState({ syncing: false, offer: false, notice: 'Synchro longue : elle continue en arrière-plan' });
        break;
      default: // skipped
        setReadOnly(false);
        onState({
          syncing: false,
          offer: e.reason === 'not_registered' && !hintDismissed(),
          notice: e.reason === 'needs_setup' ? 'Termine la configuration dans Synchrotron' : '',
        });
    }
  });
}
