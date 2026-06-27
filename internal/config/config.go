package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Bitbucket struct {
	Workspace string   `yaml:"workspace"`
	Repos     []string `yaml:"repos"`
}

type Config struct {
	Bitbucket           Bitbucket `yaml:"bitbucket"`
	SyncIntervalMinutes int       `yaml:"sync_interval_minutes"`
	Authors             []string  `yaml:"authors"`
	BitbucketEmail      string    `yaml:"-"`
	BitbucketAPIToken   string    `yaml:"-"`
}

func Load(path string) (Config, error) {
	var cfg Config

	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config file: %w", err)
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config file: %w", err)
	}

	cfg.BitbucketEmail = os.Getenv("BITBUCKET_EMAIL")
	cfg.BitbucketAPIToken = os.Getenv("BITBUCKET_API_TOKEN")

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.BitbucketEmail == "" {
		return fmt.Errorf("BITBUCKET_EMAIL env var must be set")
	}
	if c.BitbucketAPIToken == "" {
		return fmt.Errorf("BITBUCKET_API_TOKEN env var must be set")
	}
	if c.Bitbucket.Workspace == "" {
		return fmt.Errorf("bitbucket.workspace must be set")
	}
	if len(c.Bitbucket.Repos) == 0 {
		return fmt.Errorf("bitbucket.repos must contain at least one repo slug")
	}
	if c.SyncIntervalMinutes <= 0 {
		return fmt.Errorf("sync_interval_minutes must be greater than 0")
	}
	if len(c.Authors) == 0 {
		return fmt.Errorf("authors must contain at least one allowlisted author")
	}
	return nil
}
