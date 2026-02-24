; BackUper Agent Inno Setup Script
; Build: iscc /DSourceExe="C:\path\to\backuper-agent.exe" /DAppVersion="0.1.0" scripts\installer\BackUperAgent.iss

#define MyAppName "BackUper Agent"
#define MyAppExeName "backuper-agent.exe"
#ifndef AppVersion
#define AppVersion "0.1.0"
#endif
#ifndef SourceExe
#define SourceExe "backuper-agent.exe"
#endif

[Setup]
AppId={{A45C57D2-3A6E-4E9E-9A1E-BA9F4E2B6A91}
AppName={#MyAppName}
AppVersion={#AppVersion}
AppPublisher=BackUper
DefaultDirName={pf}\BackUperAgent
DefaultGroupName=BackUper Agent
OutputBaseFilename=BackUperAgentSetup
Compression=lzma
SolidCompression=yes
DisableProgramGroupPage=yes
WizardStyle=modern

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; DestName: "{#MyAppExeName}"; Flags: ignoreversion

[Dirs]
Name: "{userappdata}\BackUperAgent"

[Run]
Filename: "sc.exe"; Parameters: "stop BackUperAgent"; Flags: runhidden ignoreexitcode
Filename: "sc.exe"; Parameters: "delete BackUperAgent"; Flags: runhidden ignoreexitcode
Filename: "sc.exe"; Parameters: "create BackUperAgent binPath= ""{app}\{#MyAppExeName} --service"" start= auto DisplayName= ""BackUper Agent"""; Flags: runhidden
Filename: "sc.exe"; Parameters: "start BackUperAgent"; Flags: runhidden

[UninstallRun]
Filename: "sc.exe"; Parameters: "stop BackUperAgent"; Flags: runhidden ignoreexitcode
Filename: "sc.exe"; Parameters: "delete BackUperAgent"; Flags: runhidden ignoreexitcode

[Code]
var
  SourcePage: TInputDirWizardPage;
  TempPage: TInputDirWizardPage;
  ServerPage: TInputQueryWizardPage;
  ApiPage: TInputQueryWizardPage;

function JsonEscape(const S: string): string;
var
  I: Integer;
begin
  Result := '';
  for I := 1 to Length(S) do
  begin
    case S[I] of
      '"': Result := Result + '\"';
      '\': Result := Result + '\\';
    else
      Result := Result + S[I];
    end;
  end;
end;

procedure InitializeWizard;
var
  DefaultTemp: string;
begin
  SourcePage := CreateInputDirWizardPage(wpSelectDir,
    'Source Folder',
    'Select the folder that contains backup data.',
    'Choose the folder that will be archived by the agent.',
    False, '' );
  SourcePage.Add('Backup source folder');

  DefaultTemp := ExpandConstant('{userappdata}\BackUperAgent\tmp');
  TempPage := CreateInputDirWizardPage(SourcePage.ID,
    'Temporary Archive Folder',
    'Choose a local temp folder for archives.',
    'Default is in AppData. The agent will delete temp archives after sending.',
    False, '' );
  TempPage.Add('Temp archive folder');
  TempPage.Values[0] := DefaultTemp;

  ServerPage := CreateInputQueryWizardPage(TempPage.ID,
    'Server Address',
    'Enter receiver address (host:port).',
    'Example: 192.168.1.10:9000');
  ServerPage.Add('Server address:', False);

  ApiPage := CreateInputQueryWizardPage(ServerPage.ID,
    'Control Plane URL',
    'Enter web control plane base URL.',
    'Example: http://192.168.1.10:8080');
  ApiPage.Add('API URL:', False);
  ApiPage.Values[0] := 'http://localhost:8080';
end;

function IsValidHostPort(const S: string): Boolean;
var
  P: Integer;
  Host, Port: string;
  PortNum: Integer;
begin
  Result := False;
  P := Pos(':', S);
  if P <= 1 then Exit;
  Host := Copy(S, 1, P - 1);
  Port := Copy(S, P + 1, Length(S) - P);
  if (Host = '') or (Port = '') then Exit;
  PortNum := StrToIntDef(Port, -1);
  Result := (PortNum > 0) and (PortNum <= 65535);
end;

function IsValidUrl(const S: string): Boolean;
begin
  Result := (Pos('http://', S) = 1) or (Pos('https://', S) = 1);
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID = SourcePage.ID then
  begin
    if SourcePage.Values[0] = '' then
    begin
      MsgBox('Please select source folder.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    if not DirExists(SourcePage.Values[0]) then
    begin
      MsgBox('Source folder does not exist.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
  end;

  if CurPageID = TempPage.ID then
  begin
    if TempPage.Values[0] = '' then
    begin
      MsgBox('Please select temp folder.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
  end;

  if CurPageID = ServerPage.ID then
  begin
    if not IsValidHostPort(ServerPage.Values[0]) then
    begin
      MsgBox('Server address must be in host:port format.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
  end;

  if CurPageID = ApiPage.ID then
  begin
    if not IsValidUrl(ApiPage.Values[0]) then
    begin
      MsgBox('API URL must start with http:// or https://', mbError, MB_OK);
      Result := False;
      Exit;
    end;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  ConfigPath: string;
  TempDir: string;
  Json: string;
begin
  if CurStep = ssInstall then
  begin
    TempDir := TempPage.Values[0];
    if not DirExists(TempDir) then
      ForceDirectories(TempDir);
  end;

  if CurStep = ssPostInstall then
  begin
    ConfigPath := ExpandConstant('{userappdata}\BackUperAgent\config.json');

  Json := '{' + #13#10 +
    '  "home_dir": "' + JsonEscape(SourcePage.Values[0]) + '",' + #13#10 +
    '  "schedule_time": "03:00",' + #13#10 +
    '  "temp_archive_dir": "' + JsonEscape(TempPage.Values[0]) + '",' + #13#10 +
    '  "server_addr": "' + JsonEscape(ServerPage.Values[0]) + '",' + #13#10 +
    '  "api_key": "",' + #13#10 +
    '  "agent_id": "' + JsonEscape(ExpandConstant('{computername}')) + '",' + #13#10 +
    '  "poll_interval_seconds": 60,' + #13#10 +
    '  "api_url": "' + JsonEscape(ApiPage.Values[0]) + '",' + #13#10 +
    '  "event_buffer_path": "' + JsonEscape(ExpandConstant('{userappdata}\BackUperAgent\events.log')) + '"' + #13#10 +
    '}';

    SaveStringToFile(ConfigPath, Json, False);
  end;
end;
