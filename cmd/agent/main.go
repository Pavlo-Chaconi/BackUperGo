package main

import (
	"flag"
	"time"

	"BackUper/internal/agent"
	"BackUper/internal/config"
	"BackUper/internal/sender"
)

func main() {
	cfg, cfgPath, cfgErr := config.LoadOrCreate("BackUperAgent")
	_ = cfgPath
	_ = cfgErr

	source := flag.String("source", cfg.HomeDir, "source folder to archive")
	addr := flag.String("addr", cfg.ServerAddr, "server address host:port")
	apiKey := flag.String("apikey", cfg.APIKey, "api key (optional for now)")
	tempDir := flag.String("temp", cfg.TempArchiveDir, "temp archive directory (local)")
	archive := flag.String("archive", "", "explicit archive path (optional)")
	name := flag.String("name", "backup", "job name")
	retries := flag.Int("retries", 5, "max retries")
	apiURL := flag.String("api", cfg.APIURL, "control plane base url (http://host:port)")
	agentID := flag.String("agent", cfg.AgentID, "agent id")
	pollSec := flag.Int("poll", cfg.PollInterval, "poll interval seconds")
	once := flag.Bool("once", false, "run once and exit")
	service := flag.Bool("service", false, "run as Windows service")
	bufferPath := flag.String("event-buffer", cfg.EventBufferPath, "event buffer path (default in AppData)")
	flag.Parse()

	opts := agent.Options{
		AgentID:      *agentID,
		APIURL:       *apiURL,
		PollInterval: time.Duration(*pollSec) * time.Second,
		EventBuffer:  *bufferPath,
		Sender: sender.SenderOptions{
			RootFolder:  *source,
			ArchivePath: *archive,
			TempDir:     *tempDir,
			APIKey:      *apiKey,
			Name:        *name,
			Addr:        *addr,
			MaxRetries:  *retries,
		},
	}

	if *once {
		agent.RunOnce(opts)
		return
	}

	if *service {
		if err := agent.RunService(opts); err != nil {
			_ = err
		}
		return
	}

	agent.RunPolling(opts)
}
