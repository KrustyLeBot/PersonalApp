# Build, signing, release and auto-update

How a push to `main` becomes an update installed on the phone and on the PC. No secret value is written here.

## 1. Overview

```
git push main
   │
   ▼
GitHub Actions (.github/workflows/build.yml)
   ├─ test      UI logic tests (node --test) + Windows bridge tests (unittest)
   ├─ android   vite build → cap sync → gradlew assembleRelease (signed APK)
   ├─ windows   vite build → PyInstaller → PersonalDashboard.exe + .sha256
   └─ release   GitHub release « latest » = PersonalDashboard.apk + .exe + .exe.sha256
                                  │
                                  ▼
      Phone / PC: Settings → reads « latest » → downloads → verifies → installs
```

One workflow builds both apps, so a release always carries the same build number for both.

## 2. Versions

| Item | Value | Where |
|---|---|---|
| Build number | workflow run number (`GITHUB_RUN_NUMBER`), only ever increases | CI |
| Android `versionCode` / `versionName` | build number / `0.1.<build>` | `android/app/build.gradle` (`PERSONALDASHBOARD_BUILD`) |
| Windows version | `0.1.<build>` | `windows/personaldashboard/_build.py`, written by `build.ps1` |
| Release title | `PersonalDashboard build <N>` | `release` job |

Android only accepts an update whose `versionCode` is higher, so a rollback is done by pushing a fix forward, never by re-publishing an old commit. The run number keeps growing even if runs are deleted; it only restarts if the repository is recreated — never do that.

## 3. The CI (`build.yml`)

Triggers: push on `main`, pull request, manual run.

1. **`test`** — blocks everything if red.
2. **`android`** — uses the `release` environment only for a push on `main` (which is what gives it the signing secrets). On `main`, a missing keystore secret **fails the build** on purpose: an unsigned release is never published. On a pull request the APK is signed with the debug key, only uploaded as a workflow artifact, never published.
3. **`windows`** — builds the UI, bakes it into a single-file exe with PyInstaller, and writes the exe's SHA-256.
4. **`release`** (`main` only, `release` environment) — deletes the `latest` release and its tag, recreates it on the current commit with the three assets. The apps depend only on the title (`build N`) and the asset names.

## 4. The Android signing key

### Why it is critical
Android installs an update only if it is signed by the **same key** as the installed app. If the key changes or is lost, automatic updates stop and the app must be uninstalled (erasing its settings; the data files in `Documents/PersonalDashboard` survive) and reinstalled. If it leaks, someone can publish a fake update your phone accepts.

### Create it once
```bash
keytool -genkeypair -v -keystore personaldashboard.jks -alias personaldashboard \
        -keyalg RSA -keysize 4096 -validity 36500
base64 -w0 personaldashboard.jks      # value of the first secret below
```
The `.jks` is git-ignored: never commit it.

### GitHub setup (Settings → Environments → `release`)
- Restrict deployment branches to `main`.
- Secrets of that environment only:

| Secret | Content |
|---|---|
| `PERSONALDASHBOARD_KEYSTORE_BASE64` | the `.jks`, base64-encoded |
| `PERSONALDASHBOARD_KEYSTORE_PASSWORD` | password of the keystore file |
| `PERSONALDASHBOARD_KEY_PASSWORD` | password of the key inside (same as above if `keytool` reused it) |

The key alias is fixed to `personaldashboard` in the workflow.

### Do once, never undo
- Keep an **offline backup** of the `.jks` and both passwords (password manager + encrypted copy). GitHub cannot show a secret again.
- Enable two-factor authentication on the GitHub account.
- Nobody else may be able to edit the workflow on `main`: anyone who can could print the secrets during a run.
- Never add a step that prints secrets.

If the key is compromised there is no transparent rotation: create a new key, replace the secrets, and uninstall/reinstall on each device.

## 5. Auto-update

Both apps do the same check in the shared UI code (`ui/src/local/update.js`):

- `GET https://api.github.com/repos/KrustyLeBot/PersonalApp/releases/tags/latest` (public repository, no credentials; ~60 requests/hour/IP is plenty).
- The build number is read from the title and compared with the installed build; greater = update available. The asset is `PersonalDashboard.apk` or `PersonalDashboard.exe`.
- When: at every launch (a dot appears on the Settings tab), and via *Settings → Vérifier maintenant*.

### Android (`AppBridgePlugin.java`)
1. Download into `cache/updates/` with a progress bar; the size must match the release's.
2. `PackageManager.getPackageArchiveInfo` check: same package name and a strictly higher `versionCode`; otherwise the file is refused.
3. Hand-over to the system installer through a `FileProvider`. **Android verifies the signature**: an APK signed with another key is rejected. The « install unknown apps » permission is requested on first use.
4. App data (settings) is kept because it is the same app signed with the same key.

### Windows (`windows/personaldashboard/updater.py`, same approach as GitEnough)
1. Download to a temp file; size and SHA-256 (from `PersonalDashboard.exe.sha256`) must match, and the file must start with `MZ`. A bad download never replaces the working exe.
2. Windows cannot overwrite a running exe but can rename it: the old exe becomes `.old`, the new one takes its name, the app relaunches itself and closes. The `.old` file is removed at the next start.
3. Only works from the packaged exe (not from source).

### Known limits
- No detached signature or extra checksum on Android: security rests on Android's signature check and on control of the repository and the key.
- The Windows exe is not code-signed, so SmartScreen warns on first run. The in-app update is not affected.
- The APK contains the UI only; it is not split by CPU architecture (it is small and pure web).
- While the release is being recreated (a few seconds per build) a check may find nothing; the next one succeeds.

## 6. Procedures

**Publish a new version**: commit, `git push origin main`. About 5–10 minutes later *PersonalDashboard build N* appears. On the phone/PC: Settings → Vérifier maintenant → Télécharger et installer.

**Local Android build** (rarely needed): `cd ui && npm ci && npm run build && npx cap sync android`, then `cd ../android && ./gradlew :app:assembleRelease`. Without signing variables the APK uses the debug key and cannot update an installed release.

**Local Windows build**: `cd ui && npm run build`, then `cd ../windows && .\build.ps1`.

## 7. Troubleshooting

| Symptom | Likely cause |
|---|---|
| Build fails with « PERSONALDASHBOARD_KEYSTORE_BASE64 is missing » | Secrets not in the `release` environment, or the environment is not allowed on `main` |
| Green build but no release | Not a push on `main`, or it is a pull request |
| « App not installed » on update | Different signature (key changed): uninstall and reinstall |
| « No update » though a release exists | The title has no `build N`, or its number is not greater than the installed build |
| Install does not start (Android) | Allow « Install unknown apps » for PersonalDashboard |
| Windows: « Mise à jour impossible » | Running from source rather than the exe, or the exe's folder is read-only |
| Error after deleting the GitHub repository | The build number would restart at 1: never recreate the repository |
