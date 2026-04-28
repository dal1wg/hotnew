package distribute

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"hotnew/internal/config"
	"hotnew/internal/domain"
	"hotnew/internal/platform/logger"
)

type DingTalkDistributor struct {
	webhook        string
	client         *http.Client
	securityType   string
	keyword        string
	secret         string
	useKeyword     bool
	useSign        bool
	rateLimiter    *RateLimiter
}

type RateLimiter struct {
	mu           sync.Mutex
	rate         int
	period       time.Duration
	messageCount int
	lastReset    time.Time
}

type dingTalkMessage struct {
	MsgType  string                `json:"msgtype"`
	Markdown dingTalkMarkdown      `json:"markdown,omitempty"`
	Text     dingTalkText          `json:"text,omitempty"`
}

type dingTalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type dingTalkText struct {
	Content string `json:"content"`
}

func NewDingTalkDistributor(cfg config.DingTalkConfig) (DingTalkDistributor, error) {
	webhook := strings.TrimSpace(cfg.Webhook)
	if webhook == "" {
		return DingTalkDistributor{}, fmt.Errorf("dingtalk webhook is required")
	}
	webhook = strings.Trim(webhook, "` ")
	if !strings.HasPrefix(webhook, "https://oapi.dingtalk.com/robot/send?access_token=") {
		return DingTalkDistributor{}, fmt.Errorf("invalid dingtalk webhook format, expected: https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	securityType := strings.TrimSpace(cfg.SecurityType)
	if securityType != "" && securityType != "none" && securityType != "keyword" && securityType != "secret" && securityType != "both" {
		return DingTalkDistributor{}, fmt.Errorf("invalid security type, expected: none, keyword, secret, both")
	}
	if (securityType == "keyword" || securityType == "both") && strings.TrimSpace(cfg.Keyword) == "" {
		return DingTalkDistributor{}, fmt.Errorf("keyword is required when security type is keyword or both")
	}
	if (securityType == "secret" || securityType == "both") && strings.TrimSpace(cfg.Secret) == "" {
		return DingTalkDistributor{}, fmt.Errorf("secret is required when security type is secret or both")
	}

	rateLimit := cfg.RateLimit
	if rateLimit <= 0 {
		rateLimit = 20
	}
	rateLimitPeriod := cfg.RateLimitPeriod
	if rateLimitPeriod <= 0 {
		rateLimitPeriod = 60 * time.Second
	}

	return DingTalkDistributor{
		webhook:      webhook,
		client:       &http.Client{Timeout: timeout},
		securityType: securityType,
		keyword:      strings.TrimSpace(cfg.Keyword),
		secret:       strings.TrimSpace(cfg.Secret),
		useKeyword:   cfg.UseKeyword,
		useSign:      cfg.UseSign,
		rateLimiter: &RateLimiter{
			rate:         rateLimit,
			period:       rateLimitPeriod,
			messageCount: 0,
			lastReset:    time.Now(),
		},
	}, nil
}

func (d DingTalkDistributor) Distribute(ctx context.Context, article domain.Article) error {
	if err := d.rateLimiter.Wait(ctx); err != nil {
		return err
	}

	logger.Debug("DingTalk Distribute called for article: %s", article.Title)

	message := d.BuildMessage(article)

	useKeyword := d.securityType == "keyword" || d.securityType == "both" || d.useKeyword
	if useKeyword && d.keyword != "" {
		if !strings.Contains(message.Markdown.Text, d.keyword) {
			message.Markdown.Text += "\n\n" + d.keyword
		}
	}

	payload, err := json.Marshal(message)
	if err != nil {
		logger.Debug("Marshal message error: %v", err)
		return fmt.Errorf("marshal dingtalk message: %w", err)
	}

	useSign := d.securityType == "secret" || d.securityType == "both" || d.useSign
	requestURL := d.webhook
	if useSign && d.secret != "" {
		signedURL, err := d.generateSignedURL()
		if err != nil {
			logger.Debug("Generate signed URL error: %v", err)
			return fmt.Errorf("generate signed URL: %w", err)
		}
		requestURL = signedURL
	}

	logger.Debug("Sending request to: %s", requestURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		logger.Debug("Create request error: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		logger.Debug("Send request error: %v", err)
		return err
	}
	defer resp.Body.Close()

	logger.Debug("Response status: %s", resp.Status)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("dingtalk webhook returned status %s", resp.Status)
	}

	var result dingTalkResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.Debug("Decode response error: %v", err)
		return fmt.Errorf("decode dingtalk response: %w", err)
	}

	logger.Debug("DingTalk API response: code=%d, msg=%s", result.ErrCode, result.ErrMsg)
	if result.ErrCode != 0 {
		if result.ErrCode == 660026 {
			logger.Warn("DingTalk rate limit exceeded, will slow down")
			d.rateLimiter.messageCount = d.rateLimiter.rate
		}
		return fmt.Errorf("dingtalk API error: code=%d, msg=%s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.lastReset) > rl.period {
		rl.messageCount = 0
		rl.lastReset = now
	}

	if rl.messageCount >= rl.rate {
		waitTime := rl.period - now.Sub(rl.lastReset)
		if waitTime > 0 {
			logger.Info("DingTalk rate limit reached (%d/%d), waiting %v", rl.messageCount, rl.rate, waitTime)
				
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
				rl.messageCount = 0
				rl.lastReset = time.Now()
			}
		}
	}

	rl.messageCount++
	return nil
}

func (d DingTalkDistributor) generateSignedURL() (string, error) {
	timestamp := time.Now().UnixMilli()
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, d.secret)

	h := hmac.New(sha256.New, []byte(d.secret))
	_, err := h.Write([]byte(stringToSign))
	if err != nil {
		return "", err
	}
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	signatureEncoded := url.QueryEscape(signature)

	if strings.Contains(d.webhook, "&") {
		return d.webhook + "&timestamp=" + fmt.Sprintf("%d", timestamp) + "&sign=" + signatureEncoded, nil
	} else {
		return d.webhook + "&timestamp=" + fmt.Sprintf("%d", timestamp) + "&sign=" + signatureEncoded, nil
	}
}

type dingTalkResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (d DingTalkDistributor) BuildMessage(article domain.Article) dingTalkMessage {
	return dingTalkMessage{
		MsgType: "markdown",
		Markdown: dingTalkMarkdown{
			Title: renderDingTalkTitle(article),
			Text:  renderDingTalkMarkdown(article),
		},
	}
}

func renderDingTalkTitle(article domain.Article) string {
	title := strings.TrimSpace(article.Title)
	if title == "" {
		title = "Untitled"
	}
	return title
}

func renderDingTalkMarkdown(article domain.Article) string {
	title := strings.TrimSpace(article.Title)
	if title == "" {
		title = "Untitled"
	}
	summary := strings.TrimSpace(article.Summary)
	if summary == "" {
		summary = strings.TrimSpace(article.Content)
	}
	if summary == "" {
		summary = title
	}
	summary = compactText(summary, 320)

	lines := []string{
		fmt.Sprintf("## %s", title),
		fmt.Sprintf("> **来源：** %s", fallbackString(article.Source, "unknown")),
	}
	if summary != "" {
		lines = append(lines, "")
		lines = append(lines, summary)
	}
	if strings.TrimSpace(article.URL) != "" {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("[查看原文](%s)", strings.TrimSpace(article.URL)))
	}
	if !article.PublishedAt.IsZero() {
		lines = append(lines, "")
		// 转换为中国本地时间
		loc, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			// 如果加载时区失败，使用UTC时间
			lines = append(lines, fmt.Sprintf("> **发布时间：** %s", article.PublishedAt.Format(time.RFC3339)))
		} else {
			lines = append(lines, fmt.Sprintf("> **发布时间：** %s", article.PublishedAt.In(loc).Format("2006-01-02 15:04:05")))
		}
	}
	return strings.Join(lines, "\n")
}
