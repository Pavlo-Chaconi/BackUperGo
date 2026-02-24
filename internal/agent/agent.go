package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"BackUper/internal/sender"
)

type Options struct {
	AgentID      string
	APIURL       string
	PollInterval time.Duration
	EventBuffer  string
	Sender       sender.SenderOptions
}

type PollResponse struct {
	Action string `json:"action"`
	JobID  string `json:"job_id,omitempty"`
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

	for {
		action, jobID := poll(opts.APIURL, opts.AgentID)
		if action == "RUN" {
			runOnce(Options{
				AgentID:      opts.AgentID,
				APIURL:       opts.APIURL,
				PollInterval: opts.PollInterval,
				Sender:       opts.Sender,
			}, jobID)
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

func poll(apiURL, agentID string) (string, string) {
	if strings.TrimSpace(apiURL) == "" {
		return "WAIT", ""
	}
	url := strings.TrimRight(apiURL, "/") + "/api/agent/poll?agent_id=" + agentID
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("poll error: %v", err)
		return "WAIT", ""
	}
	defer resp.Body.Close()

	var pr PollResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		log.Printf("poll decode error: %v", err)
		return "WAIT", ""
	}
	if pr.Action == "" {
		return "WAIT", ""
	}
	return pr.Action, pr.JobID
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
