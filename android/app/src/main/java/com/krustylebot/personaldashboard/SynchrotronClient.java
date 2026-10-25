package com.krustylebot.personaldashboard;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.net.ConnectivityManager;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;

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
        private BroadcastReceiver receiver;
        boolean finished;

        Handle(Context app) {
            this.app = app;
        }

        void stop() {
            if (receiver != null) {
                try { app.unregisterReceiver(receiver); } catch (Exception ignored) { /* already gone */ }
                receiver = null;
            }
        }

        void cancel() {
            finished = true;
            stop();
            main.removeCallbacksAndMessages(null);
        }
    }

    /**
     * Asks Synchrotron to sync [localPath] now and reports the outcome (main thread). A 45 s cool-down per
     * folder avoids hammering Proton when the app is opened repeatedly.
     */
    static Handle syncOnOpen(Context context, String localPath, long timeoutMs, Listener listener) {
        final Context app = context.getApplicationContext();
        final Handle h = new Handle(app);
        final long[] pairId = {-1L};

        class Ender {
            void end(String state, int changes, boolean conflicts, String reason) {
                if (h.finished) return;
                h.finished = true;
                h.stop();
                h.main.removeCallbacksAndMessages(null);
                listener.onState(state, changes, conflicts, reason);
            }
        }
        final Ender ender = new Ender();

        if (!isInstalled(app)) { h.main.post(() -> ender.end("skipped", 0, false, "not_installed")); return h; }
        SharedPreferences prefs = app.getSharedPreferences("synchrotron_client", Context.MODE_PRIVATE);
        if (System.currentTimeMillis() - prefs.getLong("last_" + localPath, 0) < COOLDOWN_MS) {
            h.main.post(() -> ender.end("skipped", 0, false, "cooldown"));
            return h;
        }
        ConnectivityManager cm = app.getSystemService(ConnectivityManager.class);
        if (cm.getActiveNetwork() == null) { h.main.post(() -> ender.end("skipped", 0, false, "network")); return h; }

        h.receiver = new BroadcastReceiver() {
            @Override
            public void onReceive(Context c, Intent intent) {
                long id = intent.getLongExtra("pair_id", -1);
                if (pairId[0] >= 0 && id != pairId[0]) return;
                String status = intent.getStringExtra("status");
                boolean ok = "OK".equals(status) || "CONFLICTS".equals(status);
                if (ok) ender.end("done", intent.getIntExtra("changes", 0), "CONFLICTS".equals(status), null);
                else ender.end("failed", 0, false, status);
            }
        };
        // Listen before asking, so a very fast sync cannot be missed.
        ContextCompat.registerReceiver(app, h.receiver, new IntentFilter(ACTION_SYNC_FINISHED), ContextCompat.RECEIVER_EXPORTED);

        Intent request = new Intent(ACTION_SYNC_NOW).setPackage(PACKAGE).putExtra("local_path", localPath);
        app.sendOrderedBroadcast(request, null, new BroadcastReceiver() {
            @Override
            public void onReceive(Context c, Intent intent) {
                if (h.finished) return;
                Bundle extras = getResultExtras(false);
                switch (getResultCode()) {
                    case R_STARTED:
                    case R_RUNNING:
                        pairId[0] = extras != null ? extras.getLong("pair_id", -1L) : -1L;
                        prefs.edit().putLong("last_" + localPath, System.currentTimeMillis()).apply();
                        listener.onState("syncing", 0, false, null);
                        // Too slow: let the user work again but keep listening for the late result.
                        h.main.postDelayed(() -> { if (!h.finished) listener.onState("timeout", 0, false, null); }, timeoutMs);
                        h.main.postDelayed(() -> { h.finished = true; h.stop(); }, GIVE_UP_MS);
                        break;
                    case R_NOT_REGISTERED: ender.end("skipped", 0, false, "not_registered"); break;
                    case R_NEEDS_SETUP: ender.end("skipped", 0, false, "needs_setup"); break;
                    case R_NOT_LOGGED_IN: ender.end("skipped", 0, false, "not_logged_in"); break;
                    case R_FRESH: ender.end("skipped", 0, false, "fresh"); break;
                    case R_NETWORK: ender.end("skipped", 0, false, "network"); break;
                    default: ender.end("skipped", 0, false, "unavailable");
                }
            }
        }, h.main, NO_ANSWER, null, null);
        return h;
    }
}
