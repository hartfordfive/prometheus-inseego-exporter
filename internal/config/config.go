// Package config holds the exporter settings, which can be supplied through
// command-line flags, a YAML file, or both (flags take precedence).
package config

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"go.yaml.in/yaml/v3"
)

// Config is the full exporter configuration. YAML keys mirror the flag names:
// flag "-mifi.dial-timeout" is "mifi: {dial_timeout: ...}" in YAML.
type Config struct {
	Web    WebConfig    `yaml:"web"`
	Mifi   MifiConfig   `yaml:"mifi"`
	GPS    GPSConfig    `yaml:"gps"`
	Status StatusConfig `yaml:"status"`
	Log    LogConfig    `yaml:"log"`
}

type WebConfig struct {
	ListenAddress string `yaml:"listen_address"`
	TelemetryPath string `yaml:"telemetry_path"`
}

type MifiConfig struct {
	Address     string        `yaml:"address"`
	DialTimeout time.Duration `yaml:"dial_timeout"`
	ReadTimeout time.Duration `yaml:"read_timeout"`
}

type GPSConfig struct {
	Interval   time.Duration `yaml:"interval"`
	StaleAfter time.Duration `yaml:"stale_after"`
}

// StatusConfig controls scraping of the device's /srv/status JSON endpoint.
type StatusConfig struct {
	Enabled  bool          `yaml:"enabled"`
	URL      string        `yaml:"url"`
	Timeout  time.Duration `yaml:"timeout"`
	Interval time.Duration `yaml:"interval"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

// Default returns the built-in defaults.
func Default() Config {
	return Config{
		Web:    WebConfig{ListenAddress: ":9877", TelemetryPath: "/metrics"},
		Mifi:   MifiConfig{Address: "my.mifi:11010", DialTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second},
		GPS:    GPSConfig{Interval: 30 * time.Second, StaleAfter: 90 * time.Second},
		Status: StatusConfig{Enabled: true, URL: "http://my.mifi/srv/status", Timeout: 5 * time.Second, Interval: 30 * time.Second},
		Log:    LogConfig{Level: "info"},
	}
}

// bind registers one flag per Config field on fs, writing into c.
func bind(fs *flag.FlagSet, c *Config) {
	fs.StringVar(&c.Web.ListenAddress, "web.listen-address", c.Web.ListenAddress, "Address to expose metrics on.")
	fs.StringVar(&c.Web.TelemetryPath, "web.telemetry-path", c.Web.TelemetryPath, "Path under which to expose metrics.")
	fs.StringVar(&c.Mifi.Address, "mifi.address", c.Mifi.Address, "host:port of the MiFi GPS over Wi-Fi NMEA stream.")
	fs.DurationVar(&c.Mifi.DialTimeout, "mifi.dial-timeout", c.Mifi.DialTimeout, "Timeout when connecting to the NMEA stream.")
	fs.DurationVar(&c.Mifi.ReadTimeout, "mifi.read-timeout", c.Mifi.ReadTimeout, "Maximum time per GPS fetch to wait for RMC and GGA sentences.")
	fs.DurationVar(&c.GPS.Interval, "gps.interval", c.GPS.Interval, "Interval between GPS fetches from the NMEA stream.")
	fs.DurationVar(&c.GPS.StaleAfter, "gps.stale-after", c.GPS.StaleAfter, "Stop exporting fix metrics if no update is received for this long (0 disables).")
	fs.BoolVar(&c.Status.Enabled, "status.enabled", c.Status.Enabled, "Export device metrics from the /srv/status endpoint.")
	fs.StringVar(&c.Status.URL, "status.url", c.Status.URL, "URL of the MiFi /srv/status JSON endpoint.")
	fs.DurationVar(&c.Status.Timeout, "status.timeout", c.Status.Timeout, "Timeout when fetching the /srv/status endpoint.")
	fs.DurationVar(&c.Status.Interval, "status.interval", c.Status.Interval, "Interval between fetches of the /srv/status endpoint.")
	fs.StringVar(&c.Log.Level, "log.level", c.Log.Level, "Log level: debug, info, warn, error.")
}

// ErrShowVersion is returned by Parse when -version is given.
var ErrShowVersion = errors.New("version requested")

// Parse builds the configuration from args (without the program name).
// It returns ErrShowVersion if -version is set, before reading any config file.
// Precedence, lowest to highest: built-in defaults, the YAML file given by
// -config.file, flags set explicitly on the command line.
func Parse(name string, args []string, output io.Writer) (Config, error) {
	cli := Default()
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	configFile := fs.String("config.file", "", "Path to a YAML configuration file. Flags set on the command line override its values.")
	showVersion := fs.Bool("version", false, "Print the version, commit hash and build date, then exit.")
	bind(fs, &cli)
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if *showVersion {
		return Config{}, ErrShowVersion
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if *configFile == "" {
		return cli, cli.validate()
	}

	cfg := Default()
	if err := LoadFile(*configFile, &cfg); err != nil {
		return Config{}, err
	}
	// Re-apply explicitly set flags on top of the file.
	override := flag.NewFlagSet(name, flag.ContinueOnError)
	bind(override, &cfg)
	var err error
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config.file" || f.Name == "version" || err != nil {
			return
		}
		err = override.Set(f.Name, f.Value.String())
	})
	if err != nil {
		return Config{}, err
	}
	return cfg, cfg.validate()
}

func (c Config) validate() error {
	if c.GPS.Interval <= 0 {
		return errors.New("gps.interval must be greater than 0")
	}
	if c.Mifi.ReadTimeout <= 0 {
		return errors.New("mifi.read-timeout must be greater than 0")
	}
	if c.GPS.StaleAfter > 0 && c.GPS.StaleAfter <= c.GPS.Interval {
		return fmt.Errorf("gps.stale-after (%s) must be longer than gps.interval (%s), or 0 to disable", c.GPS.StaleAfter, c.GPS.Interval)
	}
	if c.Status.Enabled && c.Status.Interval <= 0 {
		return errors.New("status.interval must be greater than 0")
	}
	return nil
}

// LoadFile decodes the YAML file at path into c, leaving unset keys untouched.
// Unknown keys are rejected to catch typos.
func LoadFile(path string, c *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("parsing config file %s: %w", path, err)
	}
	return nil
}
