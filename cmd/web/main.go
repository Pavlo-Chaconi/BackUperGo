package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"BackUper/internal/store"
)

//go:embed web/**
var webFS embed.FS

func main() {
	cfg := loadWebConfig()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	st, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background()); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	assets, err := fs.Sub(webFS, "web/assets")
	if err != nil {
		log.Fatal(err)
	}

	// Static pages
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/login":
			http.ServeFileFS(w, r, sub, "login.html")
		case "/dashboard":
			http.ServeFileFS(w, r, sub, "dashboard.html")
		case "/settings":
			http.ServeFileFS(w, r, sub, "settings.html")
		case "/agents":
			http.ServeFileFS(w, r, sub, "agents.html")
		case "/schedule":
			http.ServeFileFS(w, r, sub, "schedule.html")
		default:
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				http.StripPrefix("/assets/", http.FileServer(http.FS(assets))).ServeHTTP(w, r)
			} else {
				http.NotFound(w, r)
			}
		}
	})

	// API: POST /api/enroll/script
	mux.HandleFunc("/api/enroll/script", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		token, expiresAt, err := st.CreateEnrollToken(r.Context(), cfg.EnrollTTL, cfg.EnrollMaxUses)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		apiURL := cfg.APIURL
		if apiURL == "" {
			apiURL = guessBaseURL(r)
		}
		script := renderInstallScript(installScriptParams{
			ServerAddr: cfg.ServerAddr,
			APIURL:     apiURL,
			Token:      token,
			Hash:       agentHash(cfg.AgentBinary),
		})
		writeJSON(w, map[string]any{
			"token":      token,
			"expires_at": expiresAt.UTC().Format(time.RFC3339),
			"script":     script,
		})
	})

	// API: GET /api/agent/download
	mux.HandleFunc("/api/agent/download", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, cfg.AgentBinary)
	})

	// API: POST /api/agent/enroll
	mux.HandleFunc("/api/agent/enroll", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Token    string `json:"token"`
			Hostname string `json:"hostname"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(payload.Token) == "" {
			http.Error(w, "token required", http.StatusBadRequest)
			return
		}
		if err := st.ConsumeEnrollToken(r.Context(), strings.TrimSpace(payload.Token)); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		agentID, err := st.CreateAgent(r.Context(), strings.TrimSpace(payload.Hostname))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"agent_id": agentID})
	})

	// API: POST /api/agent/heartbeat
	mux.HandleFunc("/api/agent/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			AgentID string `json:"agent_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if payload.AgentID != "" {
			_ = st.TouchAgent(r.Context(), payload.AgentID)
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// API: GET /api/agent/poll
	mux.HandleFunc("/api/agent/poll", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		agentID := r.URL.Query().Get("agent_id")
		if agentID != "" {
			_ = st.TouchAgent(r.Context(), agentID)
		}
		cfgVersion := 0
		if v := r.URL.Query().Get("config_version"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				cfgVersion = parsed
			}
		}
		if agentID != "" {
			if cfg, err := st.GetAgentConfig(r.Context(), agentID); err == nil && cfg.ConfigVersion > cfgVersion {
				config := map[string]any{}
				if cfg.ScheduleTime != "" {
					config["schedule_time"] = cfg.ScheduleTime
				}
				if cfg.PollIntervalSeconds > 0 {
					config["poll_interval_seconds"] = cfg.PollIntervalSeconds
				}
				if len(cfg.ConfigJSON) > 0 {
					var extra map[string]any
					if err := json.Unmarshal(cfg.ConfigJSON, &extra); err == nil {
						for k, v := range extra {
							config[k] = v
						}
					}
				}
				writeJSON(w, map[string]any{
					"action":         "CONFIG",
					"agent":          agentID,
					"ts":             time.Now().UTC().Format(time.RFC3339),
					"config":         config,
					"config_version": cfg.ConfigVersion,
				})
				return
			}
		}
		resp := map[string]any{
			"action": "WAIT",
			"agent":  agentID,
			"ts":     time.Now().UTC().Format(time.RFC3339),
		}
		writeJSON(w, resp)
	})

	// API: GET /api/agents
	mux.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rows, err := st.ListAgents(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out := make([]map[string]any, 0, len(rows))
		now := time.Now().UTC()
		for _, row := range rows {
			status := "offline"
			var lastSeen any = nil
			if row.LastSeenAt.Valid {
				lastSeen = row.LastSeenAt.Time.UTC().Format(time.RFC3339)
				if now.Sub(row.LastSeenAt.Time) <= 120*time.Second {
					status = "online"
				}
			}
			item := map[string]any{
				"agent_id":       row.AgentID,
				"hostname":       row.Hostname,
				"created_at":     row.CreatedAt.UTC().Format(time.RFC3339),
				"last_seen_at":   lastSeen,
				"status":         status,
				"config_version": row.ConfigVersion,
			}
			if row.ScheduleTime.Valid {
				item["schedule_time"] = row.ScheduleTime.String
			}
			if row.PollIntervalSeconds.Valid {
				item["poll_interval_seconds"] = int(row.PollIntervalSeconds.Int32)
			}
			out = append(out, item)
		}
		writeJSON(w, out)
	})

	// API: POST /api/agents/{agentID}/config
	mux.HandleFunc("/api/agents/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Extract agentID from path: /api/agents/{agentID}/config
		path := strings.TrimPrefix(r.URL.Path, "/api/agents/")
		path = strings.TrimSuffix(path, "/config")
		agentID := path
		if agentID == "" {
			http.Error(w, "agentID required", http.StatusBadRequest)
			return
		}

		var payload struct {
			ScheduleTime        string          `json:"schedule_time"`
			PollIntervalSeconds int             `json:"poll_interval_seconds"`
			ConfigJSON          json.RawMessage `json:"config_json"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		err := st.UpdateAgentConfig(r.Context(), agentID, store.AgentConfig{
			ScheduleTime:        payload.ScheduleTime,
			PollIntervalSeconds: payload.PollIntervalSeconds,
			ConfigJSON:          payload.ConfigJSON,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})

	// API: POST /api/agent/report
	mux.HandleFunc("/api/agent/report", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if agentID, _ := payload["agent_id"].(string); agentID != "" {
			_ = st.TouchAgent(r.Context(), agentID)
		}
		log.Printf("[REPORT] %+v", payload)
		writeJSON(w, map[string]any{"ok": true})
	})

	// API: POST /api/agent/event
	mux.HandleFunc("/api/agent/event", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if agentID, _ := payload["agent_id"].(string); agentID != "" {
			_ = st.TouchAgent(r.Context(), agentID)
		}
		log.Printf("[EVENT] %+v", payload)
		writeJSON(w, map[string]any{"ok": true})
	})

	log.Println("web-ui listening on http://0.0.0.0:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

type webConfig struct {
	DatabaseURL   string
	ServerAddr    string
	APIURL        string
	AgentBinary   string
	EnrollTTL     time.Duration
	EnrollMaxUses int
}

func loadWebConfig() webConfig {
	cfg := webConfig{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		ServerAddr:    os.Getenv("BACKUPER_SERVER_ADDR"),
		APIURL:        os.Getenv("BACKUPER_API_URL"),
		AgentBinary:   os.Getenv("BACKUPER_AGENT_BINARY"),
		EnrollTTL:     12 * time.Hour,
		EnrollMaxUses: 1,
	}
	if cfg.ServerAddr == "" {
		cfg.ServerAddr = "localhost:9000"
	}
	if cfg.AgentBinary == "" {
		cfg.AgentBinary = "backuper-agent.exe"
	}
	return cfg
}

type installScriptParams struct {
	ServerAddr string
	APIURL     string
	Token      string
	Hash       string
}

func renderInstallScript(p installScriptParams) string {
	return strings.Join([]string{
		"param(",
		"  [string]$InstallDir = \"$env:ProgramFiles\\BackUperAgent\",",
		"  [string]$AgentExe = \"\",",
		"  [string]$SourceFolder = \"\",",
		"  [string]$TempFolder = \"\",",
		"  [string]$ServerAddr = \"" + escapePS(p.ServerAddr) + "\",",
		"  [string]$ApiUrl = \"" + escapePS(p.APIURL) + "\",",
		"  [string]$EnrollToken = \"" + escapePS(p.Token) + "\",",
		"  [string]$AgentSha256 = \"" + escapePS(p.Hash) + "\"",
		")",
		"",
		"$ErrorActionPreference = \"Stop\"",
		"",
		"$downloadUrl = ($ApiUrl.TrimEnd('/')) + \"/api/agent/download\"",
		"$downloadPath = Join-Path $env:TEMP \"backuper-agent.exe\"",
		"",
		"function Read-Value($prompt, $default) {",
		"  $label = if ($default -ne \"\") { \"$prompt [$default]\" } else { $prompt }",
		"  $v = Read-Host $label",
		"  if ($v -eq \"\") { return $default }",
		"  return $v",
		"}",
		"",
		"Write-Host \"BackUper Agent Installer\" -ForegroundColor Cyan",
		"",
		"if (-not $AgentExe -or -not (Test-Path $AgentExe)) {",
		"  Write-Host \"Downloading agent from $downloadUrl\" -ForegroundColor Cyan",
		"  Invoke-WebRequest -Uri $downloadUrl -OutFile $downloadPath",
		"  $AgentExe = $downloadPath",
		"}",
		"if (-not (Test-Path $AgentExe)) {",
		"  throw \"Agent exe not found: $AgentExe\"",
		"}",
		"if ($AgentSha256) {",
		"  $hash = (Get-FileHash -Algorithm SHA256 -Path $AgentExe).Hash.ToLower()",
		"  if ($hash -ne $AgentSha256.ToLower()) {",
		"    throw \"Agent checksum mismatch: $hash\"",
		"  }",
		"}",
		"",
		"if (-not $SourceFolder) {",
		"  $SourceFolder = Read-Value \"Source folder with backups\" $SourceFolder",
		"}",
		"if (-not (Test-Path $SourceFolder)) {",
		"  throw \"Source folder not found: $SourceFolder\"",
		"}",
		"",
		"if (-not $TempFolder) {",
		"  $defaultTemp = Join-Path $env:APPDATA \"BackUperAgent\\tmp\"",
		"  $TempFolder = Read-Value \"Temp archive folder (local)\" $defaultTemp",
		"}",
		"",
		"if (-not $ServerAddr) {",
		"  $ServerAddr = Read-Value \"Server address (host:port)\" $ServerAddr",
		"}",
		"",
		"$null = New-Item -ItemType Directory -Force $InstallDir",
		"$null = New-Item -ItemType Directory -Force $TempFolder",
		"",
		"$exeTarget = Join-Path $InstallDir \"backuper-agent.exe\"",
		"Copy-Item -Force $AgentExe $exeTarget",
		"",
		"$configDir = Join-Path $env:APPDATA \"BackUperAgent\"",
		"$null = New-Item -ItemType Directory -Force $configDir",
		"$configPath = Join-Path $configDir \"config.json\"",
		"",
		"$bufferPath = Join-Path $env:APPDATA \"BackUperAgent\\events.log\"",
		"",
		"$agentId = \"\"",
		"if (-not $EnrollToken) {",
		"  $agentId = $env:COMPUTERNAME",
		"}",
		"",
		"$config = @{",
		"  home_dir = $SourceFolder",
		"  schedule_time = \"03:00\"",
		"  temp_archive_dir = $TempFolder",
		"  server_addr = $ServerAddr",
		"  api_key = \"\"",
		"  agent_id = $agentId",
		"  enrollment_token = $EnrollToken",
		"  poll_interval_seconds = 60",
		"  api_url = $ApiUrl",
		"  event_buffer_path = $bufferPath",
		"} | ConvertTo-Json -Depth 5",
		"",
		"$config | Set-Content -Encoding UTF8 $configPath",
		"",
		"Write-Host \"Installed to: $InstallDir\" -ForegroundColor Green",
		"Write-Host \"Config: $configPath\" -ForegroundColor Green",
		"Write-Host \"Temp folder: $TempFolder\" -ForegroundColor Green",
		"Write-Host \"Run: $exeTarget\" -ForegroundColor Yellow",
	}, "\n")
}

func escapePS(v string) string {
	return strings.ReplaceAll(v, "\"", "`\"")
}

func agentHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func guessBaseURL(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}
	return proto + "://" + r.Host
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
