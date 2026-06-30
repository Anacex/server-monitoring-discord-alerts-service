package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

type DiscordNotifier struct {
	WebhookURL    string
	StateDir      string
	CooldownHours int
	httpClient    *http.Client
}

func NewDiscordNotifier(webhookURL, stateDir string, cooldownHours int) *DiscordNotifier {
	return &DiscordNotifier{
		WebhookURL:    webhookURL,
		StateDir:      stateDir,
		CooldownHours: cooldownHours,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9]`)

// Send posts message to the Discord webhook, but skips sending if the same
// alert key was already sent within the cooldown window.
func (d *DiscordNotifier) Send(message, key string) {
	stateFile := filepath.Join(d.StateDir, unsafeChars.ReplaceAllString(key, "_"))

	if data, err := os.ReadFile(stateFile); err == nil {
		lastSentUnix, err := strconv.ParseInt(string(data), 10, 64)
		if err == nil {
			elapsed := time.Since(time.Unix(lastSentUnix, 0))
			if elapsed < time.Duration(d.CooldownHours)*time.Hour {
				log.Printf("skipping alert %q (sent %.1fh ago, cooldown %dh)", key, elapsed.Hours(), d.CooldownHours)
				return
			}
		}
	}

	if err := d.post(message); err != nil {
		log.Printf("failed to send Discord alert for %q: %v", key, err)
		return
	}

	if err := os.WriteFile(stateFile, []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0644); err != nil {
		log.Printf("warning: failed to write state file for %q: %v", key, err)
	}

	log.Printf("alert sent: %s", key)
}

func (d *DiscordNotifier) post(message string) error {
	payload, err := json.Marshal(map[string]string{"content": message})
	if err != nil {
		return err
	}

	resp, err := d.httpClient.Post(d.WebhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return &httpError{Status: resp.StatusCode}
	}
	return nil
}

type httpError struct {
	Status int
}

func (e *httpError) Error() string {
	return "discord webhook returned status " + strconv.Itoa(e.Status)
}