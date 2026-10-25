# Builds dist\PersonalDashboard.exe (single file, no console) from the already-built UI (ui\dist).
# Locally:  cd ui; npm run build; cd ..\windows; .\build.ps1
# In the CI: .\build.ps1 -Ci  (the build number comes from BUILD_NUMBER)
param([switch]$Ci)
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (-not (Test-Path ..\ui\dist\index.html)) { throw "ui\dist is missing: run 'npm run build' in ui\ first" }

$python = "python"
if (-not $Ci) {
    if (-not (Test-Path .venv)) {
        python -m venv .venv
        .\.venv\Scripts\python.exe -m pip install -r requirements.txt pyinstaller
    }
    $python = ".\.venv\Scripts\python.exe"
} else {
    python -m pip install -r requirements.txt pyinstaller
}

# The UI build and the build number are baked into the package.
Remove-Item -Recurse -Force personaldashboard\web -ErrorAction SilentlyContinue
Copy-Item -Recurse ..\ui\dist personaldashboard\web
$build = if ($env:BUILD_NUMBER) { $env:BUILD_NUMBER } else { "0" }
Set-Content -Path personaldashboard\_build.py -Value "BUILD = $build" -Encoding ascii

& $python -m PyInstaller --noconfirm --onefile --windowed --name PersonalDashboard `
    --icon personaldashboard\assets\icon.ico `
    --add-data "personaldashboard\web;personaldashboard\web" `
    --add-data "personaldashboard\assets;personaldashboard\assets" `
    --collect-submodules webview `
    --exclude-module tkinter `
    main.py
if ($LASTEXITCODE -ne 0) { throw "PyInstaller failed (is PersonalDashboard.exe still running?)" }

# Published next to the exe; the in-app update check compares the download against it.
$hash = (Get-FileHash dist\PersonalDashboard.exe -Algorithm SHA256).Hash.ToLower()
Set-Content -Path dist\PersonalDashboard.exe.sha256 -Value "$hash  PersonalDashboard.exe" -Encoding ascii
Write-Host "OK -> dist\PersonalDashboard.exe (build $build, sha256 $hash)"
