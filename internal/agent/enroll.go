package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"BackUper/internal/config"
)

type enrollRequest struct {
	Token    string `json:"token"`
	Hostname string `json:"hostname"`
}

type enrollResponse struct {
	AgentID string `json:"agent_id"`
}

func EnsureEnrolled(cfg config.Config, cfgPath string) (config.Config, error) {
	if strings.TrimSpace(cfg.AgentID) != "" {
		return cfg, nil
	}
	if strings.TrimSpace(cfg.EnrollmentToken) == "" {
		return cfg, nil
	}
	if strings.TrimSpace(cfg.APIURL) == "" {
		return cfg, fmt.Errorf("api url is empty")
	}

	host, _ := os.Hostname()
	req := enrollRequest{
		Token:    cfg.EnrollmentToken,
		Hostname: host,
	}
	body, _ := json.Marshal(req)
	url := strings.TrimRight(cfg.APIURL, "/") + "/api/agent/enroll"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return cfg, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return cfg, fmt.Errorf("enroll failed: %s", resp.Status)
	}

	var out enrollResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return cfg, err
	}
	if strings.TrimSpace(out.AgentID) == "" {
		return cfg, fmt.Errorf("enroll failed: empty agent_id")
	}

	cfg.AgentID = out.AgentID
	cfg.EnrollmentToken = ""
	if err := config.Save(cfgPath, cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
