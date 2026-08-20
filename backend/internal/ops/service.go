package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"my_feed_system/internal/account"
	"my_feed_system/internal/config"
)

var (
	ErrOpsDenied     = errors.New("ops access denied")
	ErrQueryInvalid  = errors.New("invalid log query")
	ErrObservability = errors.New("observability query failed")
)

const (
	defaultLokiURL       = "http://loki:3100"
	defaultPrometheusURL = "http://prometheus:9090"
	maxQueryRunes        = 400
	maxSinceMinutes      = 24 * 60
	maxLogLimit          = 200
)

// Service 只给测试域邮箱提供只读观测，不转发 Grafana 账号密码。
type Service struct {
	accounts *account.Service
	lokiURL  string
	promURL  string
	client   *http.Client
}

func NewService(accounts *account.Service, cfg config.OpsConfig) *Service {
	lokiURL := strings.TrimRight(strings.TrimSpace(cfg.LokiURL), "/")
	if lokiURL == "" {
		lokiURL = defaultLokiURL
	}
	promURL := strings.TrimRight(strings.TrimSpace(cfg.PrometheusURL), "/")
	if promURL == "" {
		promURL = defaultPrometheusURL
	}
	return &Service{
		accounts: accounts,
		lokiURL:  lokiURL,
		promURL:  promURL,
		client:   &http.Client{Timeout: 8 * time.Second},
	}
}

func (s *Service) Allowed(accountID uint64) (bool, error) {
	return s.accounts.HasTestEmailIdentity(accountID)
}

func (s *Service) require(accountID uint64) error {
	ok, err := s.Allowed(accountID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrOpsDenied
	}
	return nil
}

type LogLine struct {
	Time   string `json:"time"`
	Line   string `json:"line"`
	Labels string `json:"labels,omitempty"`
}

type LogsResult struct {
	Query string    `json:"query"`
	Lines []LogLine `json:"lines"`
}

type MetricsResult struct {
	QPS       *float64 `json:"qps"`
	ErrorRate *float64 `json:"error_rate"`
}

func (s *Service) QueryLogs(ctx context.Context, accountID uint64, query string, sinceMinutes int, limit int) (*LogsResult, error) {
	if err := s.require(accountID); err != nil {
		return nil, err
	}
	query, sinceMinutes, limit, err := normalizeLogQuery(query, sinceMinutes, limit)
	if err != nil {
		return nil, err
	}

	end := time.Now().UTC()
	start := end.Add(-time.Duration(sinceMinutes) * time.Minute)
	u, err := url.Parse(s.lokiURL + "/loki/api/v1/query_range")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("query", query)
	q.Set("start", strconv.FormatInt(start.UnixNano(), 10))
	q.Set("end", strconv.FormatInt(end.UnixNano(), 10))
	q.Set("limit", strconv.Itoa(limit))
	q.Set("direction", "backward")
	u.RawQuery = q.Encode()

	var payload lokiRangeResponse
	if err := s.getJSON(ctx, u.String(), &payload); err != nil {
		return nil, err
	}
	if payload.Status != "success" {
		return nil, ErrObservability
	}

	lines := flattenLoki(payload.Data.Result, limit)
	return &LogsResult{Query: query, Lines: lines}, nil
}

func (s *Service) QueryMetrics(ctx context.Context, accountID uint64) (*MetricsResult, error) {
	if err := s.require(accountID); err != nil {
		return nil, err
	}
	result := &MetricsResult{}
	if v, err := s.promQuery(ctx, `sum(rate(http_requests_total[5m]))`); err == nil {
		result.QPS = v
	}
	if v, err := s.promQuery(ctx, `sum(rate(http_requests_total{status=~"5.."}[5m])) / clamp_min(sum(rate(http_requests_total[5m])), 0.0001)`); err == nil {
		result.ErrorRate = v
	}
	return result, nil
}

func normalizeLogQuery(query string, sinceMinutes int, limit int) (string, int, int, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		query = `{service="backend"}`
	}
	if len([]rune(query)) > maxQueryRunes || !strings.HasPrefix(query, "{") {
		return "", 0, 0, ErrQueryInvalid
	}
	if sinceMinutes <= 0 {
		sinceMinutes = 60
	}
	if sinceMinutes > maxSinceMinutes {
		sinceMinutes = maxSinceMinutes
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > maxLogLimit {
		limit = maxLogLimit
	}
	return query, sinceMinutes, limit, nil
}

type lokiRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []lokiStream `json:"result"`
	} `json:"data"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

func flattenLoki(streams []lokiStream, limit int) []LogLine {
	lines := make([]LogLine, 0, limit)
	for _, stream := range streams {
		labels := compactLabels(stream.Stream)
		for _, pair := range stream.Values {
			if len(pair) < 2 {
				continue
			}
			lines = append(lines, LogLine{
				Time:   formatLokiTime(pair[0]),
				Line:   truncateRunes(pair[1], 4000),
				Labels: labels,
			})
			if len(lines) >= limit {
				return lines
			}
		}
	}
	return lines
}

func compactLabels(stream map[string]string) string {
	if len(stream) == 0 {
		return ""
	}
	parts := make([]string, 0, len(stream))
	for _, key := range []string{"service", "container", "level"} {
		if value := stream[key]; value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	return strings.Join(parts, " ")
}

func formatLokiTime(raw string) string {
	ns, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || ns <= 0 {
		return raw
	}
	return time.Unix(0, ns).In(time.Local).Format("15:04:05")
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

func (s *Service) promQuery(ctx context.Context, expr string) (*float64, error) {
	u, err := url.Parse(s.promURL + "/api/v1/query")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("query", expr)
	u.RawQuery = q.Encode()

	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Value []any `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := s.getJSON(ctx, u.String(), &payload); err != nil {
		return nil, err
	}
	if payload.Status != "success" || len(payload.Data.Result) == 0 || len(payload.Data.Result[0].Value) < 2 {
		return nil, ErrObservability
	}
	raw, _ := payload.Data.Result[0].Value[1].(string)
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *Service) getJSON(ctx context.Context, rawURL string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrObservability, err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ErrObservability
	}
	if resp.StatusCode >= 300 {
		return ErrObservability
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return ErrObservability
	}
	return nil
}
