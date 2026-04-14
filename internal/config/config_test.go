package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/astromechza/aurelianprm/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Valid(t *testing.T) {
	raw := `{
		"db_path": "./prm.db",
		"listen": ":8080",
		"auth_password_hash": "$2a$12$abc",
		"smtp": {
			"host": "smtp.example.com",
			"port": 587,
			"username": "user@example.com",
			"password": "secret",
			"from": "PRM <user@example.com>"
		},
		"reminder_email": "me@example.com",
		"reminder_send_time": "08:00",
		"reminder_timezone": "Europe/London"
	}`

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))

	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, "./prm.db", cfg.DBPath)
	assert.Equal(t, ":8080", cfg.Listen)
	assert.Equal(t, "$2a$12$abc", cfg.AuthPasswordHash)
	assert.Equal(t, "smtp.example.com", cfg.SMTP.Host)
	assert.Equal(t, 587, cfg.SMTP.Port)
	assert.Equal(t, "user@example.com", cfg.SMTP.Username)
	assert.Equal(t, "secret", cfg.SMTP.Password)
	assert.Equal(t, "PRM <user@example.com>", cfg.SMTP.From)
	assert.Equal(t, "me@example.com", cfg.ReminderEmail)
	assert.Equal(t, "08:00", cfg.ReminderSendTime)
	assert.Equal(t, "Europe/London", cfg.ReminderTimezone)
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path/config.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "opening config file")
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{not valid json}`), 0o600))

	_, err := config.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding config")
}

func TestLoad_UnknownField(t *testing.T) {
	raw := `{"db_path": "./prm.db", "unknown_field": "value"}`
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))

	_, err := config.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding config")
}
