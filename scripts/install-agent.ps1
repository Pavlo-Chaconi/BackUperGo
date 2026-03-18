param(
  [string]$InstallDir = "$env:ProgramFiles\BackUperAgent",
  [string]$AgentExe = "",
  [string]$SourceFolder = "",
  [string]$TempFolder = "",
  [string]$ServerAddr = "",
  [string]$ApiUrl = "http://localhost:8080",
  [string]$EnrollToken = ""
)

$ErrorActionPreference = "Stop"

function Read-Value($prompt, $default) {
  $label = if ($default -ne "") { "$prompt [$default]" } else { $prompt }
  $v = Read-Host $label
  if ($v -eq "") { return $default }
  return $v
}

Write-Host "BackUper Agent Installer" -ForegroundColor Cyan

if (-not $AgentExe) {
  $AgentExe = Read-Value "Path to backuper-agent.exe" $AgentExe
}
if (-not (Test-Path $AgentExe)) {
  throw "Agent exe not found: $AgentExe"
}

if (-not $SourceFolder) {
  $SourceFolder = Read-Value "Source folder with backups" $SourceFolder
}
if (-not (Test-Path $SourceFolder)) {
  throw "Source folder not found: $SourceFolder"
}

if (-not $TempFolder) {
  $defaultTemp = Join-Path $env:APPDATA "BackUperAgent\tmp"
  $TempFolder = Read-Value "Temp archive folder (local)" $defaultTemp
}

if (-not $ServerAddr) {
  $ServerAddr = Read-Value "Server address (host:port)" $ServerAddr
}

$null = New-Item -ItemType Directory -Force $InstallDir
$null = New-Item -ItemType Directory -Force $TempFolder

$exeTarget = Join-Path $InstallDir "backuper-agent.exe"
Copy-Item -Force $AgentExe $exeTarget

$configDir = Join-Path $env:APPDATA "BackUperAgent"
$null = New-Item -ItemType Directory -Force $configDir
$configPath = Join-Path $configDir "config.json"

$bufferPath = Join-Path $env:APPDATA "BackUperAgent\events.log"

$agentId = ""
if (-not $EnrollToken) {
  $agentId = $env:COMPUTERNAME
}

$config = @{
  home_dir = $SourceFolder
  schedule_time = "03:00"
  temp_archive_dir = $TempFolder
  server_addr = $ServerAddr
  api_key = ""
  agent_id = $agentId
  enrollment_token = $EnrollToken
  poll_interval_seconds = 60
  api_url = $ApiUrl
  event_buffer_path = $bufferPath
} | ConvertTo-Json -Depth 5

$config | Set-Content -Encoding UTF8 $configPath

Write-Host "Installed to: $InstallDir" -ForegroundColor Green
Write-Host "Config: $configPath" -ForegroundColor Green
Write-Host "Temp folder: $TempFolder" -ForegroundColor Green
Write-Host "Run: $exeTarget" -ForegroundColor Yellow
