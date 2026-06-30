package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	WebhookURL         string
	Domains            []string
	SSLThresholdDays   int
	DiskThresholdPct   int
	CheckIntervalMin   int
	AlertCooldownHours int
	StateDir           string
	DiskCheckPaths     []string
}

func loadConfig() Config {
	cfg := Config{
		WebhookURL:         mustEnv("DISCORD_WEBHOOK_URL"),
		Domains:            splitCSV(getEnv("SSL_DOMAINS", "")),
		SSLThresholdDays:   getEnvInt("SSL_THRESHOLD_DAYS", 14),
		DiskThresholdPct:   getEnvInt("DISK_THRESHOLD_PERCENT", 80),
		CheckIntervalMin:   getEnvInt("CHECK_INTERVAL_MINUTES", 360), // 6 hours default
		AlertCooldownHours: getEnvInt("ALERT_COOLDOWN_HOURS", 8),
		StateDir:           getEnv("STATE_DIR", "/var/lib/server-monitor"),
		DiskCheckPaths:     splitCSV(getEnv("DISK_CHECK_PATH", "/")),
	}
	return cfg
}

func main() {
	cfg := loadConfig()

	if err := os.MkdirAll(cfg.StateDir, 0755); err != nil {
		log.Fatalf("failed to create state dir: %v", err)
	}

	notifier := NewDiscordNotifier(cfg.WebhookURL, cfg.StateDir, cfg.AlertCooldownHours)

	log.Printf("server-monitor starting. domains=%v ssl_threshold=%dd disk_threshold=%d%% interval=%dm",
		cfg.Domains, cfg.SSLThresholdDays, cfg.DiskThresholdPct, cfg.CheckIntervalMin)

	for {
		runChecks(cfg, notifier)
		time.Sleep(time.Duration(cfg.CheckIntervalMin) * time.Minute)
	}
}

func runChecks(cfg Config, notifier *DiscordNotifier) {
	log.Println("--- running checks ---")

	for _, domain := range cfg.Domains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}
		result, err := CheckSSL(domain, cfg.SSLThresholdDays)
		if err != nil {
			log.Printf("SSL check failed for %s: %v", domain, err)
			notifier.Send(
				"⚠️ **SSL Check Error**\nDomain: "+domain+"\nError: "+err.Error(),
				"ssl_error_"+domain,
			)
			continue
		}
		log.Printf("SSL %s: expires %s, %d days left", domain, result.ExpiryDate.Format("2006-01-02"), result.DaysLeft)
		if result.DaysLeft <= cfg.SSLThresholdDays {
			var statusLine string
			if result.DaysLeft < 0 {
				statusLine = "\n🔴 **ALREADY EXPIRED** (" + strconv.Itoa(-result.DaysLeft) + " days ago)"
			} else {
				statusLine = "\nDays left: " + strconv.Itoa(result.DaysLeft)
			}
			msg := "⚠️ **SSL Certificate Alert**\nDomain: " + domain +
				"\nExpires: " + result.ExpiryDate.Format("2006-01-02") +
				statusLine +
				"\nThreshold: " + strconv.Itoa(cfg.SSLThresholdDays) + " days"
			notifier.Send(msg, "ssl_"+domain)
		}
	}

	diskResults, err := CheckDisk(cfg.DiskCheckPaths)
	if err != nil {
		log.Printf("disk check failed: %v", err)
		notifier.Send("⚠️ **Disk Check Error**\n"+err.Error(), "disk_error")
		return
	}

	for _, d := range diskResults {
		log.Printf("Disk %s: %d%% used", d.Mount, d.UsagePercent)
		if d.UsagePercent >= cfg.DiskThresholdPct {
			msg := "💾 **Disk Space Alert**\nMount: " + d.Mount +
				"\nUsage: " + strconv.Itoa(d.UsagePercent) + "%" +
				"\nThreshold: " + strconv.Itoa(cfg.DiskThresholdPct) + "%"
			notifier.Send(msg, "disk_"+d.Mount)
		}
	}

	log.Println("--- checks complete ---")
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return val
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("invalid int for %s=%q, using fallback %d", key, val, fallback)
		return fallback
	}
	return i
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}