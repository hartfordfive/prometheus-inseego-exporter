package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseDefaults(t *testing.T) {
	cfg, err := Parse("test", nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != Default() {
		t.Errorf("got %+v, want defaults", cfg)
	}
}

func TestParseFileAndFlagPrecedence(t *testing.T) {
	p := writeFile(t, `
web:
  listen_address: ":1234"
mifi:
  address: 192.168.1.1:11010
  dial_timeout: 10s
gps:
  stale_after: 2m
  interval: 45s
status:
  interval: 1m
  url: http://192.168.1.1/srv/status
log:
  level: debug
`)
	cfg, err := Parse("test", []string{"-config.file", p, "-mifi.address", "10.0.0.1:11010", "-log.level=warn", "-status.enabled=false"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	want := Default()
	want.Web.ListenAddress = ":1234"                  // from file
	want.Mifi.Address = "10.0.0.1:11010"              // flag overrides file
	want.Mifi.DialTimeout = 10 * time.Second          // from file
	want.GPS.StaleAfter = 2 * time.Minute             // from file
	want.Log.Level = "warn"                           // flag overrides file
	want.Status.URL = "http://192.168.1.1/srv/status" // from file
	want.Status.Enabled = false                       // flag overrides default
	want.GPS.Interval = 45 * time.Second              // from file
	want.Status.Interval = time.Minute                // from file
	if cfg != want {
		t.Errorf("got  %+v\nwant %+v", cfg, want)
	}
}

func TestParseFileErrors(t *testing.T) {
	for name, content := range map[string]string{
		"unknown key":           "mifi:\n  adress: x\n",
		"bad duration":          "mifi:\n  dial_timeout: soon\n",
		"zero interval":         "gps:\n  interval: 0s\n",
		"stale before interval": "gps:\n  interval: 2m\n  stale_after: 1m\n",
		"zero status interval":  "status:\n  interval: 0s\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse("test", []string{"-config.file", writeFile(t, content)}, io.Discard); err == nil {
				t.Error("expected error")
			}
		})
	}
	if _, err := Parse("test", []string{"-config.file", "/does/not/exist.yaml"}, io.Discard); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestShowVersion(t *testing.T) {
	// -version wins even if the config file is missing.
	_, err := Parse("test", []string{"-config.file", "/does/not/exist.yaml", "-version"}, io.Discard)
	if !errors.Is(err, ErrShowVersion) {
		t.Errorf("want ErrShowVersion, got %v", err)
	}
}

func TestEmptyFile(t *testing.T) {
	cfg, err := Parse("test", []string{"-config.file", writeFile(t, "")}, io.Discard)
	if err != nil || cfg != Default() {
		t.Errorf("got %+v, %v", cfg, err)
	}
}
