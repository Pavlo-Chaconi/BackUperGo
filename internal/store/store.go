package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	db *sql.DB
}

type AgentRow struct {
	AgentID             string
	Hostname            string
	CreatedAt           time.Time
	LastSeenAt          sql.NullTime
	ScheduleTime        sql.NullString
	PollIntervalSeconds sql.NullInt32
	ConfigJSON          json.RawMessage
	ConfigVersion       int
}

type AgentConfig struct {
	ScheduleTime        string          `json:"schedule_time,omitempty"`
	PollIntervalSeconds int             `json:"poll_interval_seconds,omitempty"`
	ConfigJSON          json.RawMessage `json:"config_json,omitempty"`
	ConfigVersion       int             `json:"config_version,omitempty"`
}

func Open(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("store is nil")
	}
	stmts := []string{
		`create table if not exists enroll_tokens (
			token text primary key,
			max_uses int not null default 1,
			used_count int not null default 0,
			expires_at timestamptz not null,
			created_at timestamptz not null default now()
		)`,
		`create table if not exists agents (
			agent_id text primary key,
			hostname text,
			created_at timestamptz not null default now(),
			last_seen_at timestamptz,
			schedule_time text,
			poll_interval_seconds int,
			config_json jsonb,
			config_version int not null default 0
		)`,
		`alter table agents add column if not exists schedule_time text`,
		`alter table agents add column if not exists poll_interval_seconds int`,
		`alter table agents add column if not exists config_json jsonb`,
		`alter table agents add column if not exists config_version int not null default 0`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateEnrollToken(ctx context.Context, ttl time.Duration, maxUses int) (string, time.Time, error) {
	if maxUses <= 0 {
		maxUses = 1
	}
	expiresAt := time.Now().Add(ttl)
	for i := 0; i < 5; i++ {
		token, err := randomToken(32)
		if err != nil {
			return "", time.Time{}, err
		}
		_, err = s.db.ExecContext(ctx, `
			insert into enroll_tokens (token, max_uses, used_count, expires_at)
			values ($1, $2, 0, $3)
		`, token, maxUses, expiresAt)
		if err == nil {
			return token, expiresAt, nil
		}
	}
	return "", time.Time{}, errors.New("failed to generate unique token")
}

func (s *Store) ConsumeEnrollToken(ctx context.Context, token string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var usedCount int
	var maxUses int
	var expiresAt time.Time
	row := tx.QueryRowContext(ctx, `
		select used_count, max_uses, expires_at
		from enroll_tokens
		where token = $1
		for update
	`, token)
	if err := row.Scan(&usedCount, &maxUses, &expiresAt); err != nil {
		return err
	}

	now := time.Now()
	if now.After(expiresAt) {
		return errors.New("token expired")
	}
	if usedCount >= maxUses {
		return errors.New("token already used")
	}

	if _, err := tx.ExecContext(ctx, `
		update enroll_tokens
		set used_count = used_count + 1
		where token = $1
	`, token); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateAgent(ctx context.Context, hostname string) (string, error) {
	for i := 0; i < 5; i++ {
		agentID, err := randomToken(18)
		if err != nil {
			return "", err
		}
		_, err = s.db.ExecContext(ctx, `
			insert into agents (agent_id, hostname)
			values ($1, $2)
		`, agentID, hostname)
		if err == nil {
			return agentID, nil
		}
	}
	return "", errors.New("failed to generate unique agent id")
}

func (s *Store) TouchAgent(ctx context.Context, agentID string) error {
	_, err := s.db.ExecContext(ctx, `
		update agents
		set last_seen_at = now()
		where agent_id = $1
	`, agentID)
	return err
}

func (s *Store) ListAgents(ctx context.Context) ([]AgentRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		select agent_id, hostname, created_at, last_seen_at, schedule_time, poll_interval_seconds, config_json, config_version
		from agents
		order by created_at desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AgentRow
	for rows.Next() {
		var row AgentRow
		if err := rows.Scan(
			&row.AgentID,
			&row.Hostname,
			&row.CreatedAt,
			&row.LastSeenAt,
			&row.ScheduleTime,
			&row.PollIntervalSeconds,
			&row.ConfigJSON,
			&row.ConfigVersion,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) GetAgentConfig(ctx context.Context, agentID string) (AgentConfig, error) {
	var cfg AgentConfig
	var scheduleTime sql.NullString
	var pollInterval sql.NullInt32
	var configJSON json.RawMessage
	var configVersion int
	err := s.db.QueryRowContext(ctx, `
		select schedule_time, poll_interval_seconds, config_json, config_version
		from agents
		where agent_id = $1
	`, agentID).Scan(&scheduleTime, &pollInterval, &configJSON, &configVersion)
	if err != nil {
		return cfg, err
	}
	cfg.ConfigVersion = configVersion
	if scheduleTime.Valid {
		cfg.ScheduleTime = scheduleTime.String
	}
	if pollInterval.Valid {
		cfg.PollIntervalSeconds = int(pollInterval.Int32)
	}
	if len(configJSON) > 0 {
		cfg.ConfigJSON = configJSON
	}
	return cfg, nil
}

func (s *Store) UpdateAgentConfig(ctx context.Context, agentID string, cfg AgentConfig) error {
	_, err := s.db.ExecContext(ctx, `
		update agents
		set schedule_time = $2,
			poll_interval_seconds = $3,
			config_json = $4,
			config_version = config_version + 1
		where agent_id = $1
	`, agentID, nullString(cfg.ScheduleTime), nullInt32(cfg.PollIntervalSeconds), cfg.ConfigJSON)
	return err
}

func nullString(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func nullInt32(v int) sql.NullInt32 {
	if v <= 0 {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(v), Valid: true}
}

func randomToken(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
