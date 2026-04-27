package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	HTTP       HTTPConfig
	Summary    SummaryConfig
	Store      StoreConfig
	Distribute DistributeConfig
	Scheduler  SchedulerConfig
	Retry      RetryConfig
	Sources    []SourceConfig
}

type HTTPConfig struct{ Addr string }

type SummaryConfig struct{ MaxChars int }

type StoreConfig struct {
	Backend                string
	DataDir                string
	SQLiteDSN              string
	FileArticlesPath       string
	FileDeliveriesPath     string
	FileRetriesPath        string
	FileRetriesArchivePath string
}

type DistributeConfig struct {
	AsyncBuffer int
	Blog        BlogConfig
	WeCom       WeComConfig
	DingTalk    DingTalkConfig
}

type SchedulerConfig struct {
	Enabled          bool
	Interval         time.Duration
	RunLimit         int
	RunTimeout       time.Duration
	StartImmediately bool
}

type RetryConfig struct {
	Enabled      bool
	Interval     time.Duration
	BatchSize    int
	Timeout      time.Duration
	MaxAttempts  int
	Backoff      time.Duration
	MaxBackoff   time.Duration
	ArchiveAfter time.Duration
	ArchiveBatch int
}

type BlogConfig struct {
	Enabled   bool
	Endpoint  string
	Timeout   time.Duration
	AuthToken string
	SiteName  string
	Author    string
	Mode      string
}

type WeComConfig struct {
	Enabled bool
	Webhook string
	Timeout time.Duration
}

type DingTalkConfig struct {
	Enabled       bool
	Webhook       string
	Timeout       time.Duration
	SecurityType  string
	Keyword       string
	Secret        string
	UseKeyword    bool
	UseSign       bool
}

type SourceConfig struct {
	Name           string
	Kind           string
	BaseURL        string
	FeedURL        string
	AccessMode     string
	LicenseNote    string
	TermsURL       string
	RateLimit      int
	Enabled        bool
	Timeout        time.Duration
	UserAgent      string
	DefaultTag     string
	FetchBatchSize int
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
			Enabled:      envBool("HOTNEW_DINGTALK_ENABLED", true),
			Webhook:      envOrDefault("HOTNEW_DINGTALK_WEBHOOK", "https://oapi.dingtalk.com/robot/send?access_token=e9158be3b0b1f0c4ecdd0e40494c5da0bf1c5830bb34434828bd83a881b9fe4b"),
			Timeout:      envDuration("HOTNEW_DINGTALK_TIMEOUT", 5*time.Second),
			SecurityType: envOrDefault("HOTNEW_DINGTALK_SECURITY_TYPE", "none"),
			Keyword:      envOrDefault("HOTNEW_DINGTALK_KEYWORD", ""),
			Secret:       envOrDefault("HOTNEW_DINGTALK_SECRET", ""),
			UseKeyword:   envBool("HOTNEW_DINGTALK_USE_KEYWORD", false),
			UseSign:      envBool("HOTNEW_DINGTALK_USE_SIGN", false),
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
		Sources: loadSources(),
	}
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
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func loadSources() []SourceConfig {
	var sources []SourceConfig

	// Load up to 10 sources
	for i := 1; i <= 10; i++ {
		sourcePrefix := fmt.Sprintf("HOTNEW_SOURCE_%d_", i)
		name := envOrDefault(sourcePrefix+"NAME", "")
		if name == "" {
			continue
		}

		source := SourceConfig{
			Name:           name,
			Kind:           envOrDefault(sourcePrefix+"KIND", "rss"),
			BaseURL:        envOrDefault(sourcePrefix+"BASE_URL", ""),
			FeedURL:        envOrDefault(sourcePrefix+"FEED_URL", ""),
			AccessMode:     envOrDefault(sourcePrefix+"ACCESS_MODE", "public_rss"),
			LicenseNote:    envOrDefault(sourcePrefix+"LICENSE_NOTE", ""),
			TermsURL:       envOrDefault(sourcePrefix+"TERMS_URL", ""),
			RateLimit:      envInt(sourcePrefix+"RATE_LIMIT", 60),
			Enabled:        envBool(sourcePrefix+"ENABLED", true),
			Timeout:        envDuration(sourcePrefix+"TIMEOUT", 8*time.Second),
			UserAgent:      envOrDefault(sourcePrefix+"USER_AGENT", "hotnew/0.1 (+compliant-rss-fetcher)"),
			DefaultTag:     envOrDefault(sourcePrefix+"DEFAULT_TAG", ""),
			FetchBatchSize: envInt(sourcePrefix+"FETCH_BATCH_SIZE", 20),
		}

		if source.Enabled {
			sources = append(sources, source)
		}
	}

	// Fallback to default sources if none are configured
	if len(sources) == 0 {
		sources = []SourceConfig{
			{Name: "google-news-world", Kind: "rss", BaseURL: "https://news.google.com", FeedURL: "https://news.google.com/rss?hl=en-US&gl=US&ceid=US:en", AccessMode: "public_rss", LicenseNote: "Only metadata and linked summaries should be redistributed after reviewing source terms.", TermsURL: "https://policies.google.com/terms", RateLimit: 60, Enabled: true, Timeout: 8 * time.Second, UserAgent: "hotnew/0.1 (+compliant-rss-fetcher)", DefaultTag: "world", FetchBatchSize: 20},
			{Name: "ithome", Kind: "rss", BaseURL: "https://www.ithome.com", FeedURL: "https://www.ithome.com/rss/", AccessMode: "public_rss", LicenseNote: "Only metadata and linked summaries should be redistributed after reviewing source terms.", TermsURL: "https://www.ithome.com", RateLimit: 60, Enabled: true, Timeout: 8 * time.Second, UserAgent: "hotnew/0.1 (+compliant-rss-fetcher)", DefaultTag: "tech-cn", FetchBatchSize: 20},
			{Name: "google-deepmind-blog", Kind: "rss", BaseURL: "https://deepmind.google", FeedURL: "https://deepmind.google/blog/rss.xml", AccessMode: "public_rss", LicenseNote: "Only metadata and linked summaries should be redistributed after reviewing source terms.", TermsURL: "https://policies.google.com/terms", RateLimit: 60, Enabled: true, Timeout: 8 * time.Second, UserAgent: "hotnew/0.1 (+compliant-rss-fetcher)", DefaultTag: "ai", FetchBatchSize: 20},
		}
	}

	return sources
}