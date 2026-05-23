package mailer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildMessage_PlainText(t *testing.T) {
	msg := buildMessage("From Name <from@example.com>", "to@example.com", "Test Subject", "Hello, world!", "text/plain")

	assert.Contains(t, msg, "From: From Name <from@example.com>\r\n")
	assert.Contains(t, msg, "To: to@example.com\r\n")
	assert.Contains(t, msg, "Subject: Test Subject\r\n")
	assert.Contains(t, msg, "Content-Type: text/plain; charset=utf-8\r\n")
	assert.Contains(t, msg, "\r\n\r\n") // blank line separating headers from body
	assert.True(t, strings.HasSuffix(msg, "Hello, world!"), "body should be at end")
}

func TestBuildMessage_HTML(t *testing.T) {
	msg := buildMessage("from@example.com", "to@example.com", "Subject", "<p>Hi</p>", "text/html")

	assert.Contains(t, msg, "Content-Type: text/html; charset=utf-8\r\n")
	assert.True(t, strings.HasSuffix(msg, "<p>Hi</p>"), "body should be at end")
}
