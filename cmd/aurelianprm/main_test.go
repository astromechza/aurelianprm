package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildVersion(t *testing.T) {
	v := buildVersion()
	assert.True(t, strings.HasPrefix(v, "aurelianprm "), "version string should start with 'aurelianprm '")
}

func TestRun_NoArgs(t *testing.T) {
	require.NoError(t, run([]string{}))
}

func TestRun_Help(t *testing.T) {
	require.NoError(t, run([]string{"--help"}))
	require.NoError(t, run([]string{"-h"}))
	require.NoError(t, run([]string{"help"}))
}

func TestRun_Version(t *testing.T) {
	require.NoError(t, run([]string{"--version"}))
	require.NoError(t, run([]string{"version"}))
}

func TestRun_UnknownCommand(t *testing.T) {
	err := run([]string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestRun_ServeNoConfig(t *testing.T) {
	err := run([]string{"serve"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--config")
}

func TestRun_ServeInvalidConfig(t *testing.T) {
	err := run([]string{"serve", "--config", "/nonexistent/config.json"})
	require.Error(t, err)
}


func TestRun_SendTestEmailNoConfig(t *testing.T) {
	err := run([]string{"send-test-email"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--config")
}

func TestRun_SendTestEmailInvalidConfig(t *testing.T) {
	err := run([]string{"send-test-email", "--config", "/nonexistent/config.json"})
	require.Error(t, err)
}

func TestRun_SendTestEmailValidConfig(t *testing.T) {
	cfgJSON := `{"db_path":":memory:","listen":":0","auth_password_hash":"$2a$12$x","smtp":{"host":"","port":0,"username":"","password":"","from":""},"reminder_email":"","reminder_send_time":"08:00","reminder_timezone":"UTC"}`
	path := t.TempDir() + "/config.json"
	require.NoError(t, os.WriteFile(path, []byte(cfgJSON), 0o600))

	err := run([]string{"send-test-email", "--config", path})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestRun_HashPassword_EmptyStdin(t *testing.T) {
	// runHashPassword reads from stdin/syscall; skip interactive test in CI.
	// The integration of bcrypt and term.ReadPassword is tested manually.
	t.Skip("interactive: requires a real terminal")
}
