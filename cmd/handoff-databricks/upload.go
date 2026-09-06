package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	filesAPIPath      = "/api/2.0/fs/files"
	statementsAPIPath = "/api/2.0/sql/statements"
	localHostScheme   = "file://"
	requestTimeout    = 60 * time.Second
)

type Config struct {
	Host        string
	Token       string
	WarehouseID string
	VolumePath  string
	Table       string
}

func ConfigFromEnv() (Config, error) {
	c := Config{
		Host:        strings.TrimSpace(os.Getenv("DATABRICKS_HOST")),
		Token:       strings.TrimSpace(os.Getenv("DATABRICKS_TOKEN")),
		WarehouseID: strings.TrimSpace(os.Getenv("DATABRICKS_WAREHOUSE_ID")),
		VolumePath:  strings.TrimSpace(os.Getenv("DATABRICKS_VOLUME_PATH")),
		Table:       strings.TrimSpace(os.Getenv("DATABRICKS_TABLE")),
	}
	if c.Table == "" {
		c.Table = "handoff_packets"
	}
	if c.Host == "" {
		return c, errors.New("DATABRICKS_HOST is required (use file:///path for local output)")
	}
	if c.IsLocal() {
		return c, nil
	}
	if c.Token == "" {
		return c, errors.New("DATABRICKS_TOKEN is required when DATABRICKS_HOST is a workspace URL")
	}
	if c.VolumePath == "" {
		return c, errors.New("DATABRICKS_VOLUME_PATH is required when DATABRICKS_HOST is a workspace URL")
	}
	return c, nil
}

func (c Config) IsLocal() bool { return strings.HasPrefix(c.Host, localHostScheme) }

func (c Config) LocalDir() string { return strings.TrimPrefix(c.Host, localHostScheme) }

type Uploader struct {
	cfg    Config
	client *http.Client
}

func NewUploader(cfg Config) *Uploader {
	return &Uploader{cfg: cfg, client: &http.Client{Timeout: requestTimeout}}
}

func objectName(p Packet, now time.Time) string {
	head := p.Head
	if head == "" {
		head = "nohead"
	}
	return fmt.Sprintf("%s-%d.json", sanitizeSegment(head), now.UTC().UnixNano())
}

func sanitizeSegment(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}

func (u *Uploader) Upload(ctx context.Context, p Packet, payload []byte, now time.Time) (string, error) {
	name := objectName(p, now)
	repoDir := sanitizeSegment(p.Repo)

	if u.cfg.IsLocal() {
		dir := filepath.Join(u.cfg.LocalDir(), repoDir)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return "", fmt.Errorf("create local output dir: %w", err)
		}
		dest := filepath.Join(dir, name)
		if err := os.WriteFile(dest, payload, 0o600); err != nil {
			return "", fmt.Errorf("write local output: %w", err)
		}
		return dest, nil
	}

	remote := path.Join(u.cfg.VolumePath, repoDir, name)
	endpoint, err := u.endpoint(filesAPIPath + ensureLeadingSlash(remote))
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+u.cfg.Token)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := u.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload to volume: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upload to volume: %s: %s", resp.Status, readSnippet(resp.Body))
	}
	return remote, nil
}

func (u *Uploader) CopyInto(ctx context.Context, remotePath string) error {
	if u.cfg.IsLocal() {
		return nil
	}
	if u.cfg.WarehouseID == "" {
		return errors.New("DATABRICKS_WAREHOUSE_ID is required to run COPY INTO")
	}

	stmt := fmt.Sprintf(
		"COPY INTO %s FROM '%s' FILEFORMAT = JSON FORMAT_OPTIONS ('inferTimestamp' = 'true') COPY_OPTIONS ('mergeSchema' = 'false')",
		u.cfg.Table, path.Dir(ensureLeadingSlash(remotePath)),
	)
	body, err := json.Marshal(map[string]any{
		"statement":    stmt,
		"warehouse_id": u.cfg.WarehouseID,
		"wait_timeout": "30s",
	})
	if err != nil {
		return fmt.Errorf("encode statement request: %w", err)
	}
	endpoint, err := u.endpoint(statementsAPIPath)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build statement request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+u.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("copy into: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("copy into: %s: %s", resp.Status, readSnippet(resp.Body))
	}
	return nil
}

func (u *Uploader) endpoint(p string) (string, error) {
	base, err := url.Parse(strings.TrimSuffix(u.cfg.Host, "/"))
	if err != nil {
		return "", fmt.Errorf("parse DATABRICKS_HOST: %w", err)
	}
	if base.Scheme != "https" && base.Host != "127.0.0.1" && !strings.HasPrefix(base.Host, "127.0.0.1:") {
		return "", errors.New("DATABRICKS_HOST must use https")
	}
	return base.String() + p, nil
}

func ensureLeadingSlash(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}

func readSnippet(r io.Reader) string {
	b, err := io.ReadAll(io.LimitReader(r, 512))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
