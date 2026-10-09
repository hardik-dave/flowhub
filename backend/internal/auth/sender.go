package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/flowos/hub/internal/config"
)

// SmsSender is the §6.4 delivery seam: ConsoleSender in dev, MSG91 in
// prod. SendOTP is fire-and-forget from the caller's perspective.
type SmsSender interface {
	SendOTP(mobile, code string) error
}

// ConsoleSender is the dev transport (SPEC §6.4). It writes the SMS
// body to a writer for a human developer to read. It is deliberately
// NOT the structured logger: AGENTS.md rule 8 forbids OTP codes in
// logs, and production always uses MSG91. See DECISIONS.md.
type ConsoleSender struct {
	Out io.Writer
}

func (c ConsoleSender) SendOTP(mobile, code string) error {
	w := c.Out
	if w == nil {
		w = os.Stdout
	}
	_, err := fmt.Fprintf(w, "[dev-sms] to=%s body=Your FlowOS verification code is %s. Valid 5 minutes.\n", mobile, code)
	return err
}

// MSG91Sender posts to the MSG91 Flow API. The DLT template variable is
// assumed to be named "code" (see DECISIONS.md).
type MSG91Sender struct {
	AuthKey    string
	SenderID   string
	TemplateID string
	Client     *http.Client
	URL        string
}

func (m *MSG91Sender) SendOTP(mobile, code string) error {
	url := m.URL
	if url == "" {
		url = "https://api.msg91.com/api/v5/flow/"
	}
	client := m.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	body, err := json.Marshal(map[string]any{
		"template_id": m.TemplateID,
		"sender":      m.SenderID,
		"short_url":   "0",
		"recipients": []map[string]string{
			{"mobiles": strings.TrimPrefix(mobile, "+"), "code": code},
		},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("authkey", m.AuthKey)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("msg91: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// NewSender picks MSG91 when it is configured, else the dev console
// sender. Production SMS requires TRAI DLT registration (SPEC §6.4).
func NewSender(cfg *config.Config) SmsSender {
	if cfg != nil && cfg.Msg91.AuthKey != "" && cfg.Msg91.DLTTemplateID != "" {
		return &MSG91Sender{
			AuthKey:    cfg.Msg91.AuthKey,
			SenderID:   cfg.Msg91.SenderID,
			TemplateID: cfg.Msg91.DLTTemplateID,
		}
	}
	return &ConsoleSender{Out: os.Stdout}
}
