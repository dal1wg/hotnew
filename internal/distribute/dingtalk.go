package distribute

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"hotnew/internal/config"
	"hotnew/internal/domain"
)

type DingTalkDistributor struct {
	webhook      string
	client       *http.Client
	securityType string
	keyword      string
	secret       string
	useKeyword   bool
	useSign      bool
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
	return DingTalkDistributor{
		webhook:      webhook,
		client:       &http.Client{Timeout: timeout},
		securityType: securityType,
		keyword:      strings.TrimSpace(cfg.Keyword),
		secret:       strings.TrimSpace(cfg.Secret),
		useKeyword:   cfg.UseKeyword,
		useSign:      cfg.UseSign,
	}, nil
}

func (d DingTalkDistributor) Distribute(ctx context.Context, article domain.Article) error {
	// 调试日志
	log.Printf("DEBUG: DingTalk Distribute called for article: %s", article.Title)
	
	message := d.BuildMessage(article)
	
	// 处理关键词安全设置（支持同时使用关键词和加签）
	// 当 securityType 为 "keyword"、"both" 或 useKeyword 为 true 时启用关键词
	useKeyword := d.securityType == "keyword" || d.securityType == "both" || d.useKeyword
	if useKeyword && d.keyword != "" {
		// 确保消息中包含关键词
		if !strings.Contains(message.Markdown.Text, d.keyword) {
			// 在消息末尾添加关键词
			message.Markdown.Text += "\n\n" + d.keyword
		}
	}
	
	payload, err := json.Marshal(message)
	if err != nil {
		log.Printf("DEBUG: Marshal message error: %v", err)
		return fmt.Errorf("marshal dingtalk message: %w", err)
	}

	// 处理加签安全设置（支持同时使用关键词和加签）
	// 当 securityType 为 "secret"、"both" 或 useSign 为 true 时启用加签
	useSign := d.securityType == "secret" || d.securityType == "both" || d.useSign
	requestURL := d.webhook
	if useSign && d.secret != "" {
		signedURL, err := d.generateSignedURL()
		if err != nil {
			log.Printf("DEBUG: Generate signed URL error: %v", err)
			return fmt.Errorf("generate signed URL: %w", err)
		}
		requestURL = signedURL
	}

	log.Printf("DEBUG: Sending request to: %s", requestURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		log.Printf("DEBUG: Create request error: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		log.Printf("DEBUG: Send request error: %v", err)
		return err
	}
	defer resp.Body.Close()

	log.Printf("DEBUG: Response status: %s", resp.Status)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("dingtalk webhook returned status %s", resp.Status)
	}

	var result dingTalkResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("DEBUG: Decode response error: %v", err)
		return fmt.Errorf("decode dingtalk response: %w", err)
	}

	log.Printf("DEBUG: DingTalk API response: code=%d, msg=%s", result.ErrCode, result.ErrMsg)
	if result.ErrCode != 0 {
		return fmt.Errorf("dingtalk API error: code=%d, msg=%s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// generateSignedURL 生成带签名的URL
func (d DingTalkDistributor) generateSignedURL() (string, error) {
	timestamp := time.Now().UnixMilli()
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, d.secret)
	
	// 计算HMAC-SHA256签名
	h := hmac.New(sha256.New, []byte(d.secret))
	_, err := h.Write([]byte(stringToSign))
	if err != nil {
		return "", err
	}
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	
	// URL编码签名
	signatureEncoded := url.QueryEscape(signature)
	
	// 构建最终URL
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
		lines = append(lines, fmt.Sprintf("> **发布时间：** %s", article.PublishedAt.Format(time.RFC3339)))
	}
	return strings.Join(lines, "\n")
}