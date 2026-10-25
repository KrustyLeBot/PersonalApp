package com.krustylebot.personaldashboard;

import android.content.BroadcastReceiver;
import android.content.ComponentName;
import android.content.ServiceConnection;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.net.ConnectivityManager;
import android.os.Bundle;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;
import android.os.Message;
import android.os.Messenger;
import android.os.RemoteException;

import androidx.core.content.ContextCompat;

/**
 * Java port of the Synchrotron client library (client/ in KrustyLeBot/Synchrotron), which syncs a folder
 * with Proton Drive. Every call degrades silently when Synchrotron is not installed.
 */
final class SynchrotronClient {
    static final String PACKAGE = "app.synchrotron";
    private static final String ACTION_REGISTER_FOLDER = "app.synchrotron.action.REGISTER_FOLDER";
    private static final String ACTION_SYNC_NOW = "app.synchrotron.action.SYNC_NOW";
    private static final String ACTION_SYNC_FINISHED = "app.synchrotron.action.SYNC_FINISHED";
    private static final String ACTION_BIND_SYNC = "app.synchrotron.action.BIND_SYNC";
    // Messages of the bound sync service (SyncService in Synchrotron).
    private static final int MSG_SYNC = 1, MSG_ANSWER = 2, MSG_FINISHED = 3;

    private static final int R_STARTED = 1, R_RUNNING = 2, R_NOT_REGISTERED = 3, R_NEEDS_SETUP = 4,
            R_NOT_LOGGED_IN = 5, R_FRESH = 6, R_NETWORK = 7;
    private static final int NO_ANSWER = -1000;
    private static final long COOLDOWN_MS = 45_000L;
    private static final long GIVE_UP_MS = 10 * 60_000L;

    /** state: syncing | done | failed | timeout | skipped. reason: only for skipped. */
    interface Listener {
        void onState(String state, int changes, boolean hasConflicts, String reason);
    }

    private SynchrotronClient() {}

    static boolean isInstalled(Context context) {
        try {
            context.getPackageManager().getPackageInfo(PACKAGE, 0);
            return true;
        } catch (PackageManager.NameNotFoundException e) {
            return false;
        }
    }

    /** Shows Synchrotron's confirmation screen for syncing [localPath]; nothing is created without the user's OK. */
    static boolean registerFolder(Context context, String localPath, String appName, String remotePath,
                                  String conflictPolicy, int intervalMinutes, boolean watchChanges, String exclusions) {
        if (!isInstalled(context)) return false;
        Intent intent = new Intent(ACTION_REGISTER_FOLDER).setPackage(PACKAGE)
                .putExtra("local_path", localPath)
                .putExtra("app_name", appName)
                .putExtra("remote_path", remotePath)
                .putExtra("conflict_policy", conflictPolicy)
                .putExtra("interval_minutes", intervalMinutes)
                .putExtra("watch_changes", watchChanges)
                .putExtra("exclusions", exclusions)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        try {
            context.startActivity(intent);
            return true;
        } catch (Exception e) {
            return false;
        }
    }

    /** Handle returned by [syncOnOpen]; [cancel] stops listening. */
    static final class Handle {
        private final Handler main = new Handler(Looper.getMainLooper());
        private final Context app;
        private Runnable stopper;
        boolean finished;

        Handle(Context app) {
            this.app = app;
        }

        void stop() {
            if (stopper != null) {
                Runnable r = stopper;
                stopper = null;
                try { r.run(); } catch (Exception ignored) { /* already gone */ }
            }
        }

        void cancel() {
            finished = true;
            stop();
            main.removeCallbacksAndMessages(null);
        }
    }

    /**
     * Asks Synchrotron to sync [localPath] now and reports the outcome (main thread). Binds Synchrotron's
     * sync service: while this app is in the foreground and bound, Synchrotron runs at the same priority,
     * so the sync starts at once. Older Synchrotron builds are reached through the SYNC_NOW broadcast.
     * A 45 s cool-down per folder avoids hammering Proton when the app is opened repeatedly.
     */
    static Handle syncOnOpen(Context context, String localPath, long timeoutMs, Listener listener) {
        final Context app = context.getApplicationContext();
        final Handle h = new Handle(app);

        final class Flow {
            void end(String state, int changes, boolean conflicts, String reason) {
                if (h.finished) return;
                h.finished = true;
                h.stop();
                h.main.removeCallbacksAndMessages(null);
                listener.onState(state, changes, conflicts, reason);
            }

            void answer(int code, SharedPreferences prefs) {
                if (h.finished) return;
                switch (code) {
                    case R_STARTED:
                    case R_RUNNING:
                        prefs.edit().putLong("last_" + localPath, System.currentTimeMillis()).apply();
                        listener.onState("syncing", 0, false, null);
                        // Too slow: let the user work again but keep listening for the late result.
                        h.main.postDelayed(() -> { if (!h.finished) listener.onState("timeout", 0, false, null); }, timeoutMs);
                        h.main.postDelayed(() -> { h.finished = true; h.stop(); }, GIVE_UP_MS);
                        break;
                    case R_NOT_REGISTERED: end("skipped", 0, false, "not_registered"); break;
                    case R_NEEDS_SETUP: end("skipped", 0, false, "needs_setup"); break;
                    case R_NOT_LOGGED_IN: end("skipped", 0, false, "not_logged_in"); break;
                    case R_FRESH: end("skipped", 0, false, "fresh"); break;
                    case R_NETWORK: end("skipped", 0, false, "network"); break;
                    default: end("skipped", 0, false, "unavailable");
                }
            }

            void finishedWith(String status, int changes) {
                boolean ok = "OK".equals(status) || "CONFLICTS".equals(status);
                if (ok) end("done", changes, "CONFLICTS".equals(status), null);
                else end("failed", 0, false, status);
            }
        }
        final Flow flow = new Flow();

        if (!isInstalled(app)) { h.main.post(() -> flow.end("skipped", 0, false, "not_installed")); return h; }
        final SharedPreferences prefs = app.getSharedPreferences("synchrotron_client", Context.MODE_PRIVATE);
        if (System.currentTimeMillis() - prefs.getLong("last_" + localPath, 0) < COOLDOWN_MS) {
            h.main.post(() -> flow.end("skipped", 0, false, "cooldown"));
            return h;
        }
        // Needs ACCESS_NETWORK_STATE; without it assume we are online rather than crash.
        boolean offline = false;
        try {
            offline = app.getSystemService(ConnectivityManager.class).getActiveNetwork() == null;
        } catch (RuntimeException ignored) { /* permission missing */ }
        if (offline) { h.main.post(() -> flow.end("skipped", 0, false, "network")); return h; }

        // --- Fast path: bound service ---
        final Messenger replies = new Messenger(new Handler(Looper.getMainLooper(), msg -> {
            if (msg.what == MSG_ANSWER) flow.answer(msg.arg1, prefs);
            else if (msg.what == MSG_FINISHED) {
                Bundle d = msg.getData();
                flow.finishedWith(d != null ? d.getString("status") : null, d != null ? d.getInt("changes", 0) : 0);
            }
            return true;
        }));
        final ServiceConnection connection = new ServiceConnection() {
            @Override
            public void onServiceConnected(ComponentName name, IBinder binder) {
                if (h.finished || binder == null) return;
                Message request = Message.obtain(null, MSG_SYNC);
                Bundle data = new Bundle();
                data.putString("local_path", localPath);
                request.setData(data);
                request.replyTo = replies;
                try {
                    new Messenger(binder).send(request);
                } catch (RemoteException e) {
                    h.stop();
                    broadcastPath(app, localPath, h, flow::answer, flow::finishedWith, prefs);
                }
            }

            // Synchrotron's process died (killed, updated): the sync is lost, do not keep the app read-only.
            @Override
            public void onServiceDisconnected(ComponentName name) {
                flow.end("failed", 0, false, "DISCONNECTED");
            }

            @Override
            public void onBindingDied(ComponentName name) {
                flow.end("failed", 0, false, "DISCONNECTED");
            }
        };
        boolean bound;
        try {
            bound = app.bindService(new Intent(ACTION_BIND_SYNC).setPackage(PACKAGE), connection,
                    Context.BIND_AUTO_CREATE | Context.BIND_IMPORTANT);
        } catch (SecurityException e) {
            bound = false;
        }
        if (bound) {
            h.stopper = () -> app.unbindService(connection);
            return h;
        }
        // --- Older Synchrotron: ordered broadcast, then SYNC_FINISHED ---
        broadcastPath(app, localPath, h, flow::answer, flow::finishedWith, prefs);
        return h;
    }

    private interface Answer { void accept(int code, SharedPreferences prefs); }

    private interface Finished { void accept(String status, int changes); }

    private static void broadcastPath(Context app, String localPath, Handle h, Answer answer, Finished finished,
                                      SharedPreferences prefs) {
        final long[] pairId = {-1L};
        final BroadcastReceiver receiver = new BroadcastReceiver() {
            @Override
            public void onReceive(Context c, Intent intent) {
                long id = intent.getLongExtra("pair_id", -1);
                if (pairId[0] >= 0 && id != pairId[0]) return;
                finished.accept(intent.getStringExtra("status"), intent.getIntExtra("changes", 0));
            }
        };
        // Listen before asking, so a very fast sync cannot be missed.
        ContextCompat.registerReceiver(app, receiver, new IntentFilter(ACTION_SYNC_FINISHED), ContextCompat.RECEIVER_EXPORTED);
        h.stopper = () -> app.unregisterReceiver(receiver);

        Intent request = new Intent(ACTION_SYNC_NOW).setPackage(PACKAGE).putExtra("local_path", localPath);
        app.sendOrderedBroadcast(request, null, new BroadcastReceiver() {
            @Override
            public void onReceive(Context c, Intent intent) {
                Bundle extras = getResultExtras(false);
                pairId[0] = extras != null ? extras.getLong("pair_id", -1L) : -1L;
                int code = getResultCode();
                answer.accept(code == NO_ANSWER ? -1 : code, prefs);
            }
        }, h.main, NO_ANSWER, null, null);
    }
}
