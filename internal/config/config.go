// Package config provides configuration loading for AurelianPRM.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// SMTP holds SMTP server connection details.
type SMTP struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
}

// Config is the top-level application configuration loaded from a JSON file.
type Config struct {
	// DBPath is the file path for the SQLite database.
	DBPath string `json:"db_path"`
	// Listen is the TCP address the HTTP server binds to (e.g. ":8080").
	Listen string `json:"listen"`
	// AuthPasswordHash is a bcrypt hash (cost 12) of the single shared password.
	AuthPasswordHash string `json:"auth_password_hash"`
	// SMTP holds outbound mail server settings.
	SMTP SMTP `json:"smtp"`
	// ReminderEmail is the address to which daily digest emails are sent.
	ReminderEmail string `json:"reminder_email"`
	// ReminderSendTime is a wall-clock time in "HH:MM" format (e.g. "08:00").
	ReminderSendTime string `json:"reminder_send_time"`
	// ReminderTimezone is a valid IANA timezone name (e.g. "Europe/London").
	ReminderTimezone string `json:"reminder_timezone"`
	// ExternalURL is the public base URL of the server (e.g. "https://prm.example.com").
	// Used to build absolute links in outbound emails.
	ExternalURL string `json:"external_url,omitempty"`
	// DisableBasicAuth disables HTTP Basic Auth when set to true.
	// For development use only — never set this in production.
	DisableBasicAuth bool `json:"disable_basic_auth,omitempty"`
}

// Load reads and decodes a JSON config file from the given path.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening config file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	var cfg Config
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decoding config: %w", err)
	}
	return &cfg, nil
}
