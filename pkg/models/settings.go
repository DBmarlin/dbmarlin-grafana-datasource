package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const (
	DefaultTimeout       = 60 * time.Second
	DefaultMaxConcurrent = 8

	SQLTextFull      = "full"
	SQLTextTruncated = "truncated"
	SQLTextNone      = "none"
)

// PluginSettings is the data source configuration. The non-secret fields come
// from jsonData; the secrets are decrypted by Grafana and never reach the
// browser.
type PluginSettings struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	UIURL    string `json:"uiUrl"`
	// TimeoutSeconds is per archiver request.
	TimeoutSeconds int `json:"timeout"`
	// MaxConcurrent limits archiver requests in flight for this data source.
	MaxConcurrent int    `json:"maxConcurrent"`
	SQLText       string `json:"sqlText"`

	Secrets *SecretPluginSettings `json:"-"`
}

type SecretPluginSettings struct {
	Password string
	CACert   string
}

func LoadPluginSettings(source backend.DataSourceInstanceSettings) (*PluginSettings, error) {
	settings := PluginSettings{}
	if len(source.JSONData) > 0 {
		if err := json.Unmarshal(source.JSONData, &settings); err != nil {
			return nil, fmt.Errorf("could not unmarshal PluginSettings json: %w", err)
		}
	}

	settings.URL = strings.TrimRight(strings.TrimSpace(settings.URL), "/")
	settings.UIURL = strings.TrimRight(strings.TrimSpace(settings.UIURL), "/")
	if settings.UIURL == "" {
		settings.UIURL = settings.URL
	}
	if settings.MaxConcurrent <= 0 {
		settings.MaxConcurrent = DefaultMaxConcurrent
	}
	switch settings.SQLText {
	case SQLTextFull, SQLTextTruncated, SQLTextNone:
	default:
		settings.SQLText = SQLTextFull
	}
	settings.Secrets = &SecretPluginSettings{
		Password: source.DecryptedSecureJSONData["password"],
		CACert:   source.DecryptedSecureJSONData["tlsCACert"],
	}
	return &settings, nil
}

// Timeout is the per-request archiver timeout.
func (s *PluginSettings) Timeout() time.Duration {
	if s.TimeoutSeconds <= 0 {
		return DefaultTimeout
	}
	return time.Duration(s.TimeoutSeconds) * time.Second
}
