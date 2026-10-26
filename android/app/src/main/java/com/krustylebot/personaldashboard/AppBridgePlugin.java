package com.krustylebot.personaldashboard;

import android.Manifest;
import android.content.Intent;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.provider.Settings;

import androidx.core.content.FileProvider;

import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.StandardCopyOption;
import java.util.Iterator;
import java.util.regex.Pattern;

/**
 * Native side of the app: data files in Documents/PersonalDashboard, HTTP requests
 * (no CORS), and the self-update (download, verify, hand over to the system installer).
 */
@CapacitorPlugin(name = "AppBridge")
public class AppBridgePlugin extends Plugin {
    private static final String DATA_FOLDER = "Documents/PersonalDashboard";
    private static final Pattern SAFE_NAME = Pattern.compile("[A-Za-z0-9_.-]+");
    private static final String UPDATE_FILE = "update.apk";

    private File dataDir() {
        return new File(Environment.getExternalStorageDirectory(), DATA_FOLDER);
    }

    private boolean hasAccess() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            return Environment.isExternalStorageManager();
        }
        return getContext().checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE)
                == PackageManager.PERMISSION_GRANTED;
    }

    private long versionCode(PackageInfo info) {
        return Build.VERSION.SDK_INT >= Build.VERSION_CODES.P ? info.getLongVersionCode() : info.versionCode;
    }

    private File updateFile() {
        File dir = new File(getContext().getCacheDir(), "updates");
        //noinspection ResultOfMethodCallIgnored
        dir.mkdirs();
        return new File(dir, UPDATE_FILE);
    }

    private SynchrotronClient.Handle syncHandle;

    /**
     * Asks Synchrotron (Proton Drive sync) for a sync now and reports it through "syncState" events:
     * {state: syncing | done | failed | timeout | skipped, changes, conflicts, reason}.
     */
    @PluginMethod
    public void syncOnOpen(PluginCall call) {
        if (syncHandle != null) syncHandle.cancel();
        syncHandle = SynchrotronClient.syncOnOpen(getContext(), dataDir().getAbsolutePath(), 20_000,
                (state, changes, conflicts, reason) -> {
                    JSObject e = new JSObject();
                    e.put("state", state);
                    e.put("changes", changes);
                    e.put("conflicts", conflicts);
                    if (reason != null) e.put("reason", reason);
                    notifyListeners("syncState", e);
                });
        call.resolve();
    }

    @Override
    protected void handleOnDestroy() {
        if (syncHandle != null) syncHandle.cancel();
    }

    @PluginMethod
    public void getInfo(PluginCall call) {
        try {
            PackageInfo info = getContext().getPackageManager().getPackageInfo(getContext().getPackageName(), 0);
            JSObject ret = new JSObject();
            ret.put("build", versionCode(info));
            ret.put("version", info.versionName);
            ret.put("dataDir", dataDir().getAbsolutePath());
            ret.put("hasAccess", hasAccess());
            call.resolve(ret);
        } catch (PackageManager.NameNotFoundException e) {
            call.reject(e.getMessage());
        }
    }

    @PluginMethod
    public void requestAccess(PluginCall call) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            Uri uri = Uri.parse("package:" + getContext().getPackageName());
            Intent intent = new Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION, uri);
            try {
                getActivity().startActivity(intent);
            } catch (Exception e) {
                getActivity().startActivity(new Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION));
            }
        } else {
            getActivity().requestPermissions(new String[]{
                    Manifest.permission.READ_EXTERNAL_STORAGE,
                    Manifest.permission.WRITE_EXTERNAL_STORAGE}, 1);
        }
        call.resolve();
    }

    private File dataFile(PluginCall call) {
        String name = call.getString("name", "");
        if (!SAFE_NAME.matcher(name).matches() || name.contains("..")) {
            call.reject("Nom de fichier invalide");
            return null;
        }
        if (!hasAccess()) {
            call.reject("Accès aux fichiers non autorisé");
            return null;
        }
        return new File(dataDir(), name);
    }

    @PluginMethod
    public void readFile(PluginCall call) {
        File file = dataFile(call);
        if (file == null) return;
        JSObject ret = new JSObject();
        try {
            if (file.isFile()) {
                ByteArrayOutputStream buf = new ByteArrayOutputStream();
                try (InputStream in = new FileInputStream(file)) {
                    byte[] chunk = new byte[16 * 1024];
                    int n;
                    while ((n = in.read(chunk)) != -1) buf.write(chunk, 0, n);
                }
                ret.put("content", buf.toString("UTF-8"));
            }
            call.resolve(ret);
        } catch (IOException e) {
            call.reject("Lecture impossible : " + e.getMessage());
        }
    }

    // Written to a temporary file then renamed, so a crash never leaves a half-written file.
    @PluginMethod
    public void writeFile(PluginCall call) {
        File file = dataFile(call);
        if (file == null) return;
        String content = call.getString("content", "");
        File dir = dataDir();
        File tmp = new File(dir, file.getName() + ".tmp");
        try {
            if (!dir.isDirectory() && !dir.mkdirs()) throw new IOException("dossier non créable : " + dir);
            try (FileOutputStream out = new FileOutputStream(tmp)) {
                out.write(content.getBytes(StandardCharsets.UTF_8));
                out.getFD().sync();
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                Files.move(tmp.toPath(), file.toPath(), StandardCopyOption.REPLACE_EXISTING);
            } else {
                //noinspection ResultOfMethodCallIgnored
                file.delete();
                if (!tmp.renameTo(file)) throw new IOException("renommage impossible");
            }
            call.resolve();
        } catch (IOException e) {
            //noinspection ResultOfMethodCallIgnored
            tmp.delete();
            call.reject("Écriture impossible : " + e.getMessage());
        }
    }

    private HttpURLConnection open(String url, JSObject headers) throws IOException {
        HttpURLConnection conn = (HttpURLConnection) new URL(url).openConnection();
        conn.setConnectTimeout(15000);
        conn.setReadTimeout(30000);
        conn.setInstanceFollowRedirects(true);
        if (headers != null) {
            Iterator<String> keys = headers.keys();
            while (keys.hasNext()) {
                String key = keys.next();
                conn.setRequestProperty(key, headers.optString(key));
            }
        }
        return conn;
    }

    @PluginMethod
    public void httpGet(PluginCall call) {
        String url = call.getString("url");
        if (url == null || !url.startsWith("https://")) {
            call.reject("Seules les URL https sont autorisées");
            return;
        }
        HttpURLConnection conn = null;
        try {
            conn = open(url, call.getObject("headers"));
            int status = conn.getResponseCode();
            InputStream in = status >= 400 ? conn.getErrorStream() : conn.getInputStream();
            ByteArrayOutputStream buf = new ByteArrayOutputStream();
            if (in != null) {
                byte[] chunk = new byte[16 * 1024];
                int n;
                while ((n = in.read(chunk)) != -1) buf.write(chunk, 0, n);
                in.close();
            }
            JSObject ret = new JSObject();
            ret.put("status", status);
            ret.put("body", buf.toString("UTF-8"));
            call.resolve(ret);
        } catch (IOException e) {
            call.reject("Requête impossible : " + e.getMessage());
        } finally {
            if (conn != null) conn.disconnect();
        }
    }

    @PluginMethod
    public void downloadUpdate(PluginCall call) {
        String url = call.getString("url");
        long expected = call.getLong("size", 0L);
        if (url == null || !url.startsWith("https://")) {
            call.reject("Seules les URL https sont autorisées");
            return;
        }
        File target = updateFile();
        HttpURLConnection conn = null;
        try {
            conn = open(url, null);
            long total = conn.getContentLengthLong() > 0 ? conn.getContentLengthLong() : expected;
            long done = 0;
            long lastNotify = 0;
            try (InputStream in = conn.getInputStream(); OutputStream out = new FileOutputStream(target)) {
                byte[] chunk = new byte[64 * 1024];
                int n;
                while ((n = in.read(chunk)) != -1) {
                    out.write(chunk, 0, n);
                    done += n;
                    long now = System.currentTimeMillis();
                    if (now - lastNotify > 150) {
                        lastNotify = now;
                        JSObject progress = new JSObject();
                        progress.put("done", done);
                        progress.put("total", total);
                        notifyListeners("updateProgress", progress);
                    }
                }
            }
            if (expected > 0 && done != expected) {
                throw new IOException("téléchargement incomplet (" + done + " sur " + expected + " octets)");
            }
            verifyPackage(target);
            call.resolve();
        } catch (IOException e) {
            //noinspection ResultOfMethodCallIgnored
            target.delete();
            call.reject("Téléchargement impossible : " + e.getMessage());
        } finally {
            if (conn != null) conn.disconnect();
        }
    }

    // Same package and a strictly higher version; Android itself checks the signature at install time.
    private void verifyPackage(File apk) throws IOException {
        PackageManager pm = getContext().getPackageManager();
        PackageInfo archive = pm.getPackageArchiveInfo(apk.getAbsolutePath(), 0);
        if (archive == null) throw new IOException("fichier APK illisible");
        if (!getContext().getPackageName().equals(archive.packageName)) {
            throw new IOException("l'APK n'est pas celui de cette app");
        }
        try {
            PackageInfo current = pm.getPackageInfo(getContext().getPackageName(), 0);
            if (versionCode(archive) <= versionCode(current)) throw new IOException("l'APK n'est pas plus récent");
        } catch (PackageManager.NameNotFoundException e) {
            throw new IOException(e.getMessage());
        }
    }

    @PluginMethod
    public void installUpdate(PluginCall call) {
        File apk = updateFile();
        if (!apk.isFile()) {
            call.reject("Aucune mise à jour téléchargée");
            return;
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O
                && !getContext().getPackageManager().canRequestPackageInstalls()) {
            Intent settings = new Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                    Uri.parse("package:" + getContext().getPackageName()));
            getActivity().startActivity(settings);
            call.reject("Autorise « Installer des apps inconnues » pour PersonalDashboard, puis réessaie.");
            return;
        }
        Uri uri = FileProvider.getUriForFile(getContext(), getContext().getPackageName() + ".fileprovider", apk);
        Intent intent = new Intent(Intent.ACTION_VIEW);
        intent.setDataAndType(uri, "application/vnd.android.package-archive");
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_ACTIVITY_NEW_TASK);
        getActivity().startActivity(intent);
        call.resolve();
    }
}
