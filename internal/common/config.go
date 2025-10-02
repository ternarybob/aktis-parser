package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Parser  ParserConfig  `toml:"parser"`
	Scraper ScraperConfig `toml:"scraper"`
	Storage StorageConfig `toml:"storage"`
	Logging LoggingConfig `toml:"logging"`
}

type ParserConfig struct {
	Name        string `toml:"name"`
	Environment string `toml:"environment"`
	Port        int    `toml:"port"`
}

type ScraperConfig struct {
	AuthMethod     string           `toml:"auth_method"`
	BaseURL        string           `toml:"base_url"`
	TimeoutSeconds int              `toml:"timeout_seconds"`
	RateLimitMs    int              `toml:"rate_limit_ms"`
	Targets        TargetsConfig    `toml:"targets"`
	Jira           JiraConfig       `toml:"jira"`
	Confluence     ConfluenceConfig `toml:"confluence"`
}

type TargetsConfig struct {
	ScrapeJira       bool `toml:"scrape_jira"`
	ScrapeConfluence bool `toml:"scrape_confluence"`
}

type JiraConfig struct {
	MaxResultsPerPage int `toml:"max_results_per_page"`
}

type ConfluenceConfig struct {
	MaxResultsPerPage int `toml:"max_results_per_page"`
}

type StorageConfig struct {
	DatabasePath  string `toml:"database_path"`
	RetentionDays int    `toml:"retention_days"`
}

type LoggingConfig struct {
	Level      string `toml:"level"`
	Format     string `toml:"format"`
	Output     string `toml:"output"`
	MaxSize    int    `toml:"max_size"`
	MaxBackups int    `toml:"max_backups"`
}

func DefaultConfig() *Config {
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	execName := filepath.Base(execPath)
	execName = execName[:len(execName)-len(filepath.Ext(execName))]

	defaultDBPath := filepath.Join(execDir, "scraper.db")

	return &Config{
		Parser: ParserConfig{
			Name:        execName,
			Environment: "development",
			Port:        8080,
		},
		Scraper: ScraperConfig{
			AuthMethod:     "extension",
			BaseURL:        "https://your-company.atlassian.net",
			TimeoutSeconds: 30,
			RateLimitMs:    500,
			Targets: TargetsConfig{
				ScrapeJira:       true,
				ScrapeConfluence: true,
			},
			Jira: JiraConfig{
				MaxResultsPerPage: 50,
			},
			Confluence: ConfluenceConfig{
				MaxResultsPerPage: 25,
			},
		},
		Storage: StorageConfig{
			DatabasePath:  defaultDBPath,
			RetentionDays: 90,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "text",
			Output:     "both",
			MaxSize:    100,
			MaxBackups: 3,
		},
	}
}

func LoadConfig(configFile string) (*Config, error) {
	config := DefaultConfig()

	if configFile == "" {
		execPath, _ := os.Executable()
		execDir := filepath.Dir(execPath)
		execName := filepath.Base(execPath)
		execName = execName[:len(execName)-len(filepath.Ext(execName))]

		possiblePaths := []string{
			filepath.Join(execDir, execName+".toml"),
			filepath.Join(execDir, "config.toml"),
			"config.toml",
			"aktis-parser.toml",
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				configFile = path
				break
			}
		}

		if configFile == "" {
			return config, nil
		}
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	if err := toml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	_applyEnvOverrides(config)

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

func _applyEnvOverrides(config *Config) {
	if dbPath := os.Getenv("DATABASE_PATH"); dbPath != "" {
		config.Storage.DatabasePath = dbPath
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}

	if port := os.Getenv("SERVER_PORT"); port != "" {
		if portNum, err := strconv.Atoi(port); err == nil {
			config.Parser.Port = portNum
		}
	}

	if baseURL := os.Getenv("SCRAPER_BASE_URL"); baseURL != "" {
		config.Scraper.BaseURL = baseURL
	}
}

func (c *Config) Validate() error {
	if c.Storage.DatabasePath == "" {
		return fmt.Errorf("storage database_path is required")
	}

	if c.Parser.Port <= 0 {
		c.Parser.Port = 8080
	}

	validLogLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
	validLevel := false
	for _, level := range validLogLevels {
		if c.Logging.Level == level {
			validLevel = true
			break
		}
	}
	if !validLevel {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	validOutputs := []string{"console", "file", "both"}
	validOutput := false
	for _, output := range validOutputs {
		if c.Logging.Output == output {
			validOutput = true
			break
		}
	}
	if !validOutput {
		return fmt.Errorf("invalid log output: %s", c.Logging.Output)
	}

	if c.Scraper.TimeoutSeconds <= 0 {
		c.Scraper.TimeoutSeconds = 30
	}

	if c.Scraper.RateLimitMs < 0 {
		c.Scraper.RateLimitMs = 0
	}

	return nil
}

func (c *Config) IsProduction() bool {
	return c.Parser.Environment == "production"
}
