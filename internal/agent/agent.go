package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"BackUper/internal/config"
	"BackUper/internal/sender"
)

type Options struct {
	AgentID       string
	APIURL        string
	PollInterval  time.Duration
	EventBuffer   string
	ConfigPath    string
	ConfigVersion int
	Sender        sender.SenderOptions
}

type PollResponse struct {
	Action        string       `json:"action"`
	JobID         string       `json:"job_id,omitempty"`
	Config        *AgentConfig `json:"config,omitempty"`
	ConfigVersion int          `json:"config_version,omitempty"`
}

type AgentConfig struct {
	HomeDir             string `json:"home_dir,omitempty"`
	ScheduleTime        string `json:"schedule_time,omitempty"`
	TempArchiveDir      string `json:"temp_archive_dir,omitempty"`
	ServerAddr          string `json:"server_addr,omitempty"`
	PollIntervalSeconds int    `json:"poll_interval_seconds,omitempty"`
	APIURL              string `json:"api_url,omitempty"`
	ConfigVersion       int    `json:"config_version,omitempty"`
}

type ReportRequest struct {
	AgentID    string    `json:"agent_id"`
	JobID      string    `json:"job_id,omitempty"`
	Status     string    `json:"status"`
	Message    string    `json:"message,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

type EventRequest struct {
	AgentID string       `json:"agent_id"`
	JobID   string       `json:"job_id,omitempty"`
	Event   sender.Event `json:"event"`
}

func normalizeOptions(opts Options) Options {
	if strings.TrimSpace(opts.AgentID) == "" {
		if host, err := os.Hostname(); err == nil && host != "" {
			opts.AgentID = host
		} else {
			opts.AgentID = "agent-unknown"
		}
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 60 * time.Second
	}
	if strings.TrimSpace(opts.EventBuffer) == "" {
		if dir, err := os.UserConfigDir(); err == nil && dir != "" {
			opts.EventBuffer = filepath.Join(dir, "BackUperAgent", "events.log")
		}
	}
	return opts
}

func RunOnce(opts Options) {
	opts = normalizeOptions(opts)
	heartbeat(opts.APIURL, opts.AgentID)
	runOnce(opts, "")
}

func RunPolling(opts Options) {
	opts = normalizeOptions(opts)
	runPolling(opts, nil)
}

func runOnce(opts Options, jobID string) {
	if strings.TrimSpace(jobID) == "" {
		jobID = time.Now().UTC().Format("20060102T150405Z")
	}
	opts.Sender.EventSink = bufferedSink{
		apiURL:     opts.APIURL,
		agentID:    opts.AgentID,
		jobID:      jobID,
		bufferPath: opts.EventBuffer,
	}
	started := time.Now()
	err := sender.BuildAndSendArchive(opts.Sender)
	finished := time.Now()
	status := "OK"
	msg := ""
	if err != nil {
		status = "FAIL"
		msg = err.Error()
		log.Printf("send failed: %v", err)
	}
	report(opts.APIURL, ReportRequest{
		AgentID:    opts.AgentID,
		JobID:      jobID,
		Status:     status,
		Message:    msg,
		StartedAt:  started,
		FinishedAt: finished,
	})
}

func runPolling(opts Options, stop <-chan struct{}) {
	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()

	heartbeat(opts.APIURL, opts.AgentID)
	for {
		action, jobID, cfg := poll(opts.APIURL, opts.AgentID, opts.ConfigVersion)
		if action == "RUN" {
			runOnce(Options{
				AgentID:       opts.AgentID,
				APIURL:        opts.APIURL,
				PollInterval:  opts.PollInterval,
				ConfigPath:    opts.ConfigPath,
				ConfigVersion: opts.ConfigVersion,
				Sender:        opts.Sender,
			}, jobID)
		} else if action == "CONFIG" && cfg != nil {
			prevInterval := opts.PollInterval
			if err := applyConfig(&opts, *cfg); err == nil && cfg.ConfigVersion > 0 {
				opts.ConfigVersion = cfg.ConfigVersion
			}
			if opts.PollInterval != prevInterval {
				ticker.Stop()
				ticker = time.NewTicker(opts.PollInterval)
			}
		}
		select {
		case <-ticker.C:
		case <-stop:
			return
		}
	}
}

type eventSink struct {
	apiURL  string
	agentID string
	jobID   string
}

func (s eventSink) Emit(event sender.Event) {
	if strings.TrimSpace(s.apiURL) == "" {
		return
	}
	url := strings.TrimRight(s.apiURL, "/") + "/api/agent/event"
	body, _ := json.Marshal(EventRequest{
		AgentID: s.agentID,
		JobID:   s.jobID,
		Event:   event,
	})
	_, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("event report error: %v", err)
	}
}

type bufferedSink struct {
	apiURL     string
	agentID    string
	jobID      string
	bufferPath string
}

func (s bufferedSink) Emit(event sender.Event) {
	s.appendToBuffer(event)
	s.flushBuffer()
}

func (s bufferedSink) appendToBuffer(event sender.Event) {
	if strings.TrimSpace(s.bufferPath) == "" {
		return
	}
	dir := filepath.Dir(s.bufferPath)
	_ = os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(s.bufferPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("event buffer open error: %v", err)
		return
	}
	defer f.Close()
	line, _ := json.Marshal(EventRequest{
		AgentID: s.agentID,
		JobID:   s.jobID,
		Event:   event,
	})
	_, _ = f.Write(append(line, '\n'))
}

func (s bufferedSink) flushBuffer() {
	if strings.TrimSpace(s.bufferPath) == "" || strings.TrimSpace(s.apiURL) == "" {
		return
	}
	file, err := os.Open(s.bufferPath)
	if err != nil {
		return
	}
	defer file.Close()

	var remaining []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if err := s.postLine(line); err != nil {
			remaining = append(remaining, line)
			for scanner.Scan() {
				remaining = append(remaining, scanner.Text())
			}
			break
		}
	}

	if len(remaining) == 0 {
		_ = os.Remove(s.bufferPath)
		return
	}

	tmp := s.bufferPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(remaining, "\n")+"\n"), 0644); err == nil {
		_ = os.Rename(tmp, s.bufferPath)
	}
}

func (s bufferedSink) postLine(line string) error {
	url := strings.TrimRight(s.apiURL, "/") + "/api/agent/event"
	_, err := http.Post(url, "application/json", bytes.NewReader([]byte(line)))
	if err != nil {
		log.Printf("event report error: %v", err)
	}
	return err
}

func poll(apiURL, agentID string, configVersion int) (string, string, *AgentConfig) {
	if strings.TrimSpace(apiURL) == "" {
		return "WAIT", "", nil
	}
	url := strings.TrimRight(apiURL, "/") + "/api/agent/poll?agent_id=" + agentID + "&config_version=" + strconv.Itoa(configVersion)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("poll error: %v", err)
		return "WAIT", "", nil
	}
	defer resp.Body.Close()

	var pr PollResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		log.Printf("poll decode error: %v", err)
		return "WAIT", "", nil
	}
	if pr.Action == "" {
		return "WAIT", "", nil
	}
	if pr.Config != nil && pr.ConfigVersion > 0 {
		pr.Config.ConfigVersion = pr.ConfigVersion
	}
	return pr.Action, pr.JobID, pr.Config
}

func report(apiURL string, req ReportRequest) {
	if strings.TrimSpace(apiURL) == "" {
		return
	}
	url := strings.TrimRight(apiURL, "/") + "/api/agent/report"
	body, _ := json.Marshal(req)
	_, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("report error: %v", err)
	}
}

func heartbeat(apiURL, agentID string) {
	if strings.TrimSpace(apiURL) == "" || strings.TrimSpace(agentID) == "" {
		return
	}
	url := strings.TrimRight(apiURL, "/") + "/api/agent/heartbeat"
	body, _ := json.Marshal(map[string]string{"agent_id": agentID})
	_, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("heartbeat error: %v", err)
	}
}

func applyConfig(opts *Options, cfg AgentConfig) error {
	if opts == nil {
		return nil
	}
	if strings.TrimSpace(cfg.HomeDir) != "" {
		opts.Sender.RootFolder = cfg.HomeDir
	}
	if strings.TrimSpace(cfg.TempArchiveDir) != "" {
		opts.Sender.TempDir = cfg.TempArchiveDir
	}
	if strings.TrimSpace(cfg.ServerAddr) != "" {
		opts.Sender.Addr = cfg.ServerAddr
	}
	if strings.TrimSpace(cfg.APIURL) != "" {
		opts.APIURL = cfg.APIURL
	}
	if cfg.PollIntervalSeconds > 0 {
		opts.PollInterval = time.Duration(cfg.PollIntervalSeconds) * time.Second
	}

	if strings.TrimSpace(opts.ConfigPath) == "" {
		return nil
	}

	data, err := os.ReadFile(opts.ConfigPath)
	current := config.Config{}
	if err == nil {
		_ = json.Unmarshal(data, &current)
	}
	if strings.TrimSpace(cfg.HomeDir) != "" {
		current.HomeDir = cfg.HomeDir
	}
	if strings.TrimSpace(cfg.ScheduleTime) != "" {
		current.ScheduleTime = cfg.ScheduleTime
	}
	if strings.TrimSpace(cfg.TempArchiveDir) != "" {
		current.TempArchiveDir = cfg.TempArchiveDir
	}
	if strings.TrimSpace(cfg.ServerAddr) != "" {
		current.ServerAddr = cfg.ServerAddr
	}
	if cfg.PollIntervalSeconds > 0 {
		current.PollInterval = cfg.PollIntervalSeconds
	}
	if strings.TrimSpace(cfg.APIURL) != "" {
		current.APIURL = cfg.APIURL
	}
	if cfg.ConfigVersion > 0 {
		current.ConfigVersion = cfg.ConfigVersion
	}
	if current.AgentID == "" {
		current.AgentID = opts.AgentID
	}
	if err := config.Save(opts.ConfigPath, current); err != nil {
		return err
	}
	return nil
}
