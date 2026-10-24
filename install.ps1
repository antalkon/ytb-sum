# ytb-sum installer for Windows.
#
#   irm https://raw.githubusercontent.com/antalkon/ytb-sum/main/install.ps1 | iex
#
# Environment: YTB_SUM_VERSION (default: latest), YTB_SUM_INSTALL_DIR
# (default: %LOCALAPPDATA%\Programs\ytb-sum).
$ErrorActionPreference = 'Stop'
$repo = 'antalkon/ytb-sum'

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$version = $env:YTB_SUM_VERSION
if (-not $version) {
    $version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
}
$num = $version.TrimStart('v')
$archive = "ytb-sum_${num}_windows_${arch}.zip"
$base = "https://github.com/$repo/releases/download/$version"

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("ytb-sum-install-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    Write-Host "Downloading ytb-sum $version (windows/$arch)"
    Invoke-WebRequest "$base/$archive" -OutFile "$tmp\$archive" -UseBasicParsing
    Invoke-WebRequest "$base/checksums.txt" -OutFile "$tmp\checksums.txt" -UseBasicParsing

    $line = Select-String -Path "$tmp\checksums.txt" -Pattern " $([regex]::Escape($archive))$"
    if (-not $line) { throw "no checksum for $archive" }
    $expected = ($line.Line -split '\s+')[0]
    $actual = (Get-FileHash "$tmp\$archive" -Algorithm SHA256).Hash.ToLower()
    if ($expected -ne $actual) { throw "checksum mismatch for $archive" }

    Expand-Archive "$tmp\$archive" -DestinationPath "$tmp\x" -Force
    $dir = if ($env:YTB_SUM_INSTALL_DIR) { $env:YTB_SUM_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\ytb-sum' }
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Copy-Item "$tmp\x\ytb-sum.exe" "$dir\ytb-sum.exe" -Force
    Write-Host "Installed $dir\ytb-sum.exe"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $dir) {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$dir", 'User')
        Write-Host "Added $dir to your user PATH (restart the terminal)"
    }
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

if (-not (Get-Command yt-dlp -ErrorAction SilentlyContinue)) {
    Write-Warning 'yt-dlp is required at runtime: winget install yt-dlp.yt-dlp  (and Gyan.FFmpeg for --audio)'
}
if (-not $env:OPENROUTER_API_KEY) {
    Write-Warning 'Set your key: setx OPENROUTER_API_KEY sk-or-...  (https://openrouter.ai/keys)'
}
