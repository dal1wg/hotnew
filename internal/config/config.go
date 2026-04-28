package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"")

		if key != "" && os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

type Config struct {
	HTTP       HTTPConfig       `yaml:"http"`
	Summary    SummaryConfig    `yaml:"summary"`
	Store      StoreConfig      `yaml:"store"`
	Distribute DistributeConfig `yaml:"distribute"`
	Scheduler  SchedulerConfig   `yaml:"scheduler"`
	Retry      RetryConfig      `yaml:"retry"`
	Logging    LoggingConfig    `yaml:"logging"`
	Sources    []SourceConfig   `yaml:"sources"`
}

type HTTPConfig struct{ Addr string `yaml:"addr"` }

type SummaryConfig struct{ MaxChars int `yaml:"max_chars"` }

type StoreConfig struct {
	Backend                string `yaml:"backend"`
	DataDir                string `yaml:"data_dir"`
	SQLiteDSN              string `yaml:"sqlite_dsn"`
	FileArticlesPath       string `yaml:"file_articles_path"`
	FileDeliveriesPath     string `yaml:"file_deliveries_path"`
	FileRetriesPath        string `yaml:"file_retries_path"`
	FileRetriesArchivePath string `yaml:"file_retries_archive_path"`
}

type DistributeConfig struct {
	AsyncBuffer int          `yaml:"async_buffer"`
	Blog        BlogConfig   `yaml:"blog"`
	WeCom       WeComConfig  `yaml:"wecom"`
	DingTalk    DingTalkConfig `yaml:"dingtalk"`
}

type SchedulerConfig struct {
	Enabled          bool          `yaml:"enabled"`
	Interval         time.Duration `yaml:"interval"`
	RunLimit         int           `yaml:"run_limit"`
	RunTimeout       time.Duration `yaml:"run_timeout"`
	StartImmediately bool          `yaml:"start_immediately"`
}

type RetryConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Interval     time.Duration `yaml:"interval"`
	BatchSize    int           `yaml:"batch_size"`
	Timeout      time.Duration `yaml:"timeout"`
	MaxAttempts  int           `yaml:"max_attempts"`
	Backoff      time.Duration `yaml:"backoff"`
	MaxBackoff   time.Duration `yaml:"max_backoff"`
	ArchiveAfter time.Duration `yaml:"archive_after"`
	ArchiveBatch int           `yaml:"archive_batch"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	Output     string `yaml:"output"`
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
}

type BlogConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Endpoint  string        `yaml:"endpoint"`
	Timeout   time.Duration `yaml:"timeout"`
	AuthToken string        `yaml:"auth_token"`
	SiteName  string        `yaml:"site_name"`
	Author    string        `yaml:"author"`
	Mode      string        `yaml:"mode"`
}

type WeComConfig struct {
	Enabled bool          `yaml:"enabled"`
	Webhook string        `yaml:"webhook"`
	Timeout time.Duration `yaml:"timeout"`
}

type DingTalkConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Webhook         string        `yaml:"webhook"`
	Timeout         time.Duration `yaml:"timeout"`
	SecurityType    string        `yaml:"security_type"`
	Keyword         string        `yaml:"keyword"`
	Secret          string        `yaml:"secret"`
	UseKeyword      bool          `yaml:"use_keyword"`
	UseSign         bool          `yaml:"use_sign"`
	RateLimit       int           `yaml:"rate_limit"`
	RateLimitPeriod time.Duration `yaml:"rate_limit_period"`
}

type SourceConfig struct {
	Name           string        `yaml:"name"`
	Kind           string        `yaml:"kind"`
	BaseURL        string        `yaml:"base_url"`
	FeedURL        string        `yaml:"feed_url"`
	AccessMode     string        `yaml:"access_mode"`
	LicenseNote    string        `yaml:"license_note"`
	TermsURL       string        `yaml:"terms_url"`
	RateLimit      int           `yaml:"rate_limit"`
	Enabled        bool          `yaml:"enabled"`
	Timeout        time.Duration `yaml:"timeout"`
	UserAgent      string        `yaml:"user_agent"`
	DefaultTag     string        `yaml:"default_tag"`
	FetchBatchSize int           `yaml:"fetch_batch_size"`
}

func Load() Config {
	_ = loadEnvFile(filepath.Join(".", "hotnew.env"))

	dataDir := envOrDefault("HOTNEW_DATA_DIR", "data")
	return Config{
		HTTP:    HTTPConfig{Addr: envOrDefault("HOTNEW_HTTP_ADDR", ":8080")},
		Summary: SummaryConfig{MaxChars: envInt("HOTNEW_SUMMARY_MAX_CHARS", 180)},
		Store: StoreConfig{
			Backend:                envOrDefault("HOTNEW_STORE_BACKEND", "sqlite"),
			DataDir:                dataDir,
			SQLiteDSN:              envOrDefault("HOTNEW_SQLITE_DSN", filepath.Join(dataDir, "hotnew.db")),
			FileArticlesPath:       envOrDefault("HOTNEW_FILE_ARTICLES_PATH", filepath.Join(dataDir, "articles.jsonl")),
			FileDeliveriesPath:     envOrDefault("HOTNEW_FILE_DELIVERIES_PATH", filepath.Join(dataDir, "deliveries.jsonl")),
			FileRetriesPath:        envOrDefault("HOTNEW_FILE_RETRIES_PATH", filepath.Join(dataDir, "retry_jobs.jsonl")),
			FileRetriesArchivePath: envOrDefault("HOTNEW_FILE_RETRIES_ARCHIVE_PATH", filepath.Join(dataDir, "retry_jobs.archive.jsonl")),
		},
		Distribute: DistributeConfig{AsyncBuffer: envInt("HOTNEW_DISTRIBUTE_BUFFER", 128), Blog: BlogConfig{
			Enabled:   envBool("HOTNEW_BLOG_ENABLED", false),
			Endpoint:  strings.TrimSpace(os.Getenv("HOTNEW_BLOG_ENDPOINT")),
			Timeout:   envDuration("HOTNEW_BLOG_TIMEOUT", 5*time.Second),
			AuthToken: strings.TrimSpace(os.Getenv("HOTNEW_BLOG_AUTH_TOKEN")),
			SiteName:  envOrDefault("HOTNEW_BLOG_SITE_NAME", "hotnew"),
			Author:    envOrDefault("HOTNEW_BLOG_AUTHOR", "hotnew"),
			Mode:      envOrDefault("HOTNEW_BLOG_MODE", "markdown"),
		}, WeCom: WeComConfig{
			Enabled: envBool("HOTNEW_WECOM_ENABLED", true),
			Webhook: envOrDefault("HOTNEW_WECOM_WEBHOOK", "https://work.weixin.qq.com/wework_admin/common/openBotProfile/241d4adcd6c5849bb0de9d5b710f9e3f57"),
			Timeout: envDuration("HOTNEW_WECOM_TIMEOUT", 5*time.Second),
		}, DingTalk: DingTalkConfig{
			Enabled:         envBool("HOTNEW_DINGTALK_ENABLED", true),
			Webhook:         envOrDefault("HOTNEW_DINGTALK_WEBHOOK", "https://oapi.dingtalk.com/robot/send?access_token=e9158be3b0b1f0c4ecdd0e40494c5da0bf1c5830bb34434828bd83a881b9fe4b"),
			Timeout:         envDuration("HOTNEW_DINGTALK_TIMEOUT", 5*time.Second),
			SecurityType:    envOrDefault("HOTNEW_DINGTALK_SECURITY_TYPE", "none"),
			Keyword:         envOrDefault("HOTNEW_DINGTALK_KEYWORD", ""),
			Secret:          envOrDefault("HOTNEW_DINGTALK_SECRET", ""),
			UseKeyword:      envBool("HOTNEW_DINGTALK_USE_KEYWORD", false),
			UseSign:         envBool("HOTNEW_DINGTALK_USE_SIGN", false),
			RateLimit:       envInt("HOTNEW_DINGTALK_RATE_LIMIT", 20),
			RateLimitPeriod: envDuration("HOTNEW_DINGTALK_RATE_LIMIT_PERIOD", 60*time.Second),
		}},
		Scheduler: SchedulerConfig{
			Enabled:          envBool("HOTNEW_SCHEDULER_ENABLED", false),
			Interval:         envDuration("HOTNEW_SCHEDULER_INTERVAL", 15*time.Minute),
			RunLimit:         envInt("HOTNEW_SCHEDULER_RUN_LIMIT", 10),
			RunTimeout:       envDuration("HOTNEW_SCHEDULER_RUN_TIMEOUT", 30*time.Second),
			StartImmediately: envBool("HOTNEW_SCHEDULER_START_IMMEDIATELY", false),
		},
		Retry: RetryConfig{
			Enabled:      envBool("HOTNEW_RETRY_ENABLED", true),
			Interval:     envDuration("HOTNEW_RETRY_INTERVAL", 1*time.Minute),
			BatchSize:    envInt("HOTNEW_RETRY_BATCH_SIZE", 10),
			Timeout:      envDuration("HOTNEW_RETRY_TIMEOUT", 30*time.Second),
			MaxAttempts:  envInt("HOTNEW_RETRY_MAX_ATTEMPTS", 3),
			Backoff:      envDuration("HOTNEW_RETRY_BACKOFF", 5*time.Minute),
			MaxBackoff:   envDuration("HOTNEW_RETRY_MAX_BACKOFF", 6*time.Hour),
			ArchiveAfter: envDuration("HOTNEW_RETRY_ARCHIVE_AFTER", 168*time.Hour),
			ArchiveBatch: envInt("HOTNEW_RETRY_ARCHIVE_BATCH", 100),
		},
		Logging: LoggingConfig{
			Level:      envOrDefault("HOTNEW_LOG_LEVEL", "info"),
			Format:     envOrDefault("HOTNEW_LOG_FORMAT", "text"),
			Output:     envOrDefault("HOTNEW_LOG_OUTPUT", "stdout"),
			FilePath:   envOrDefault("HOTNEW_LOG_FILE_PATH", "logs/hotnew.log"),
			MaxSize:    envInt("HOTNEW_LOG_MAX_SIZE", 10),
			MaxBackups: envInt("HOTNEW_LOG_MAX_BACKUPS", 5),
			MaxAge:     envInt("HOTNEW_LOG_MAX_AGE", 30),
			Compress:   envBool("HOTNEW_LOG_COMPRESS", true),
		},
		Sources: loadSources(),
	}
}

func LoadFromFile(filePath string) (Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return Config{}, fmt.Errorf("open config file: %w", err)
	}
	defer file.Close()

	var cfg Config
	if err := yaml.NewDecoder(file).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config file: %w", err)
	}

	if cfg.HTTP.Addr == "" {
		cfg.HTTP.Addr = ":8080"
	}
	if cfg.Summary.MaxChars == 0 {
		cfg.Summary.MaxChars = 180
	}
	if cfg.Store.Backend == "" {
		cfg.Store.Backend = "sqlite"
	}
	if cfg.Store.DataDir == "" {
		cfg.Store.DataDir = "data"
	}
	if cfg.Store.SQLiteDSN == "" {
		cfg.Store.SQLiteDSN = filepath.Join(cfg.Store.DataDir, "hotnew.db")
	}
	if cfg.Store.FileArticlesPath == "" {
		cfg.Store.FileArticlesPath = filepath.Join(cfg.Store.DataDir, "articles.jsonl")
	}
	if cfg.Store.FileDeliveriesPath == "" {
		cfg.Store.FileDeliveriesPath = filepath.Join(cfg.Store.DataDir, "deliveries.jsonl")
	}
	if cfg.Store.FileRetriesPath == "" {
		cfg.Store.FileRetriesPath = filepath.Join(cfg.Store.DataDir, "retry_jobs.jsonl")
	}
	if cfg.Store.FileRetriesArchivePath == "" {
		cfg.Store.FileRetriesArchivePath = filepath.Join(cfg.Store.DataDir, "retry_jobs.archive.jsonl")
	}
	if cfg.Distribute.AsyncBuffer == 0 {
		cfg.Distribute.AsyncBuffer = 128
	}
	if cfg.Scheduler.Interval == 0 {
		cfg.Scheduler.Interval = 15 * time.Minute
	}
	if cfg.Scheduler.RunLimit == 0 {
		cfg.Scheduler.RunLimit = 10
	}
	if cfg.Scheduler.RunTimeout == 0 {
		cfg.Scheduler.RunTimeout = 30 * time.Second
	}
	if cfg.Retry.Interval == 0 {
		cfg.Retry.Interval = 1 * time.Minute
	}
	if cfg.Retry.BatchSize == 0 {
		cfg.Retry.BatchSize = 10
	}
	if cfg.Retry.Timeout == 0 {
		cfg.Retry.Timeout = 30 * time.Second
	}
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry.MaxAttempts = 3
	}
	if cfg.Retry.Backoff == 0 {
		cfg.Retry.Backoff = 5 * time.Minute
	}
	if cfg.Retry.MaxBackoff == 0 {
		cfg.Retry.MaxBackoff = 6 * time.Hour
	}
	if cfg.Retry.ArchiveAfter == 0 {
		cfg.Retry.ArchiveAfter = 168 * time.Hour
	}
	if cfg.Retry.ArchiveBatch == 0 {
		cfg.Retry.ArchiveBatch = 100
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "text"
	}
	if cfg.Logging.Output == "" {
		cfg.Logging.Output = "stdout"
	}
	if cfg.Logging.FilePath == "" {
		cfg.Logging.FilePath = "logs/hotnew.log"
	}
	if cfg.Logging.MaxSize == 0 {
		cfg.Logging.MaxSize = 10
	}
	if cfg.Logging.MaxBackups == 0 {
		cfg.Logging.MaxBackups = 5
	}
	if cfg.Logging.MaxAge == 0 {
		cfg.Logging.MaxAge = 30
	}
	if cfg.Distribute.DingTalk.RateLimit == 0 {
		cfg.Distribute.DingTalk.RateLimit = 20
	}
	if cfg.Distribute.DingTalk.RateLimitPeriod == 0 {
		cfg.Distribute.DingTalk.RateLimitPeriod = 60 * time.Second
	}

	if len(cfg.Sources) == 0 {
		cfg.Sources = []SourceConfig{
			{Name: "google-news-world", Kind: "rss", BaseURL: "https://news.google.com", FeedURL: "https://news.google.com/rss?hl=en-US&gl=US&ceid=US:en", AccessMode: "public_rss", LicenseNote: "Only metadata and linked summaries should be redistributed after reviewing source terms.", TermsURL: "https://policies.google.com/terms", RateLimit: 60, Enabled: true, Timeout: 8 * time.Second, UserAgent: "hotnew/0.1 (+compliant-rss-fetcher)", DefaultTag: "world", FetchBatchSize: 20},
			{Name: "ithome", Kind: "rss", BaseURL: "https://www.ithome.com", FeedURL: "https://www.ithome.com/rss/", AccessMode: "public_rss", LicenseNote: "Only metadata and linked summaries should be redistributed after reviewing source terms.", TermsURL: "https://www.ithome.com", RateLimit: 60, Enabled: true, Timeout: 8 * time.Second, UserAgent: "hotnew/0.1 (+compliant-rss-fetcher)", DefaultTag: "tech-cn", FetchBatchSize: 20},
			{Name: "google-deepmind-blog", Kind: "rss", BaseURL: "https://deepmind.google", FeedURL: "https://deepmind.google/blog/rss.xml", AccessMode: "public_rss", LicenseNote: "Only metadata and linked summaries should be redistributed after reviewing source terms.", TermsURL: "https://policies.google.com/terms", RateLimit: 60, Enabled: true, Timeout: 8 * time.Second, UserAgent: "hotnew/0.1 (+compliant-rss-fetcher)", DefaultTag: "ai", FetchBatchSize: 20},
		}
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return strings.ToLower(value) == "true" || value == "1"
}
func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}
func loadSources() []SourceConfig {
	var sources []SourceConfig
	i := 1
	for {
		name := os.Getenv(fmt.Sprintf("HOTNEW_SOURCE_%d_NAME", i))
		if name == "" {
			break
		}
		sources = append(sources, SourceConfig{
			Name:           name,
			Kind:           envOrDefault(fmt.Sprintf("HOTNEW_SOURCE_%d_KIND", i), "rss"),
			BaseURL:        envOrDefault(fmt.Sprintf("HOTNEW_SOURCE_%d_BASE_URL", i), ""),
			FeedURL:        envOrDefault(fmt.Sprintf("HOTNEW_SOURCE_%d_FEED_URL", i), ""),
			AccessMode:     envOrDefault(fmt.Sprintf("HOTNEW_SOURCE_%d_ACCESS_MODE", i), "public_rss"),
			LicenseNote:    envOrDefault(fmt.Sprintf("HOTNEW_SOURCE_%d_LICENSE_NOTE", i), ""),
			TermsURL:       envOrDefault(fmt.Sprintf("HOTNEW_SOURCE_%d_TERMS_URL", i), ""),
			RateLimit:      envInt(fmt.Sprintf("HOTNEW_SOURCE_%d_RATE_LIMIT", i), 60),
			Enabled:        envBool(fmt.Sprintf("HOTNEW_SOURCE_%d_ENABLED", i), true),
			Timeout:        envDuration(fmt.Sprintf("HOTNEW_SOURCE_%d_TIMEOUT", i), 8*time.Second),
			UserAgent:      envOrDefault(fmt.Sprintf("HOTNEW_SOURCE_%d_USER_AGENT", i), "hotnew/0.1 (+compliant-rss-fetcher)"),
			DefaultTag:     envOrDefault(fmt.Sprintf("HOTNEW_SOURCE_%d_DEFAULT_TAG", i), ""),
			FetchBatchSize: envInt(fmt.Sprintf("HOTNEW_SOURCE_%d_FETCH_BATCH_SIZE", i), 20),
		})
		i++
	}
	return sources
}
