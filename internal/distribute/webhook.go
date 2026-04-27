package distribute

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hotnew/internal/domain"
)

type WebhookDistributor struct {
	endpoint  string
	authToken string
	client    *http.Client
}

func NewWebhookDistributor(endpoint, authToken string, timeout time.Duration) (WebhookDistributor, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return WebhookDistributor{}, fmt.Errorf("webhook endpoint is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return WebhookDistributor{
		endpoint:  endpoint,
		authToken: strings.TrimSpace(authToken),
		client: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (d WebhookDistributor) Distribute(ctx context.Context, article domain.Article) error {
	payload, err := json.Marshal(article)
	if err != nil {
		return fmt.Errorf("marshal article for webhook: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if d.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+d.authToken)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %s", resp.Status)
	}
	return nil
}
