package wallet

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"my_feed_system/internal/config"
)

// Gateway 是充值支付通道。测试可注入假实现，避免打到支付宝。
type Gateway interface {
	Precreate(req PagePayRequest) (string, error)
	Query(outTradeNo string) (TradeQuery, error)
	VerifyNotify(form url.Values) (Notify, error)
}

type PagePayRequest struct {
	OutTradeNo  string
	TotalAmount string
	Subject     string
	NotifyURL   string
	ReturnURL   string
}

type TradeQuery struct {
	OutTradeNo  string
	TradeNo     string
	TradeStatus string
	TotalAmount string
}

type Notify struct {
	AppID       string
	OutTradeNo  string
	TradeNo     string
	TradeStatus string
	TotalAmount string
}

const (
	tradeSuccess  = "TRADE_SUCCESS"
	tradeFinished = "TRADE_FINISHED"
	tradeClosed   = "TRADE_CLOSED"
	waitBuyerPay  = "WAIT_BUYER_PAY"
)

type AlipayClient struct {
	cfg        config.AlipayConfig
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	httpClient *http.Client
	now        func() time.Time
}

func NewAlipayClient(cfg config.AlipayConfig) (*AlipayClient, error) {
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.PrivateKey) == "" || strings.TrimSpace(cfg.AlipayPublicKey) == "" {
		return nil, ErrPayNotConfigured
	}
	priv, err := parseRSAPrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("parse alipay private key: %w", err)
	}
	pub, err := parseRSAPublicKey(cfg.AlipayPublicKey)
	if err != nil {
		return nil, fmt.Errorf("parse alipay public key: %w", err)
	}
	gateway := strings.TrimSpace(cfg.Gateway)
	if gateway == "" {
		cfg.Gateway = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
	}
	return &AlipayClient{
		cfg:        cfg,
		privateKey: priv,
		publicKey:  pub,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		now:        time.Now,
	}, nil
}

func (c *AlipayClient) Precreate(req PagePayRequest) (string, error) {
	// 沙箱电脑网站支付跳转页会 302 到 /error（HTTP 500）。订单码预下单走 OpenAPI JSON，同一套密钥是通的。
	biz, err := json.Marshal(map[string]string{
		"out_trade_no": req.OutTradeNo,
		"total_amount": req.TotalAmount,
		"subject":      req.Subject,
	})
	if err != nil {
		return "", err
	}
	body, err := c.signedPost("alipay.trade.precreate", string(biz), map[string]string{
		"notify_url": firstNonEmpty(req.NotifyURL, c.cfg.NotifyURL),
	})
	if err != nil {
		return "", err
	}
	var wrapped struct {
		Response struct {
			Code   string `json:"code"`
			Msg    string `json:"msg"`
			SubMsg string `json:"sub_msg"`
			QRCode string `json:"qr_code"`
		} `json:"alipay_trade_precreate_response"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return "", fmt.Errorf("decode alipay precreate: %w", err)
	}
	if wrapped.Response.Code != "10000" || strings.TrimSpace(wrapped.Response.QRCode) == "" {
		return "", fmt.Errorf("alipay precreate %s: %s %s", wrapped.Response.Code, wrapped.Response.Msg, wrapped.Response.SubMsg)
	}
	return wrapped.Response.QRCode, nil
}

func (c *AlipayClient) Query(outTradeNo string) (TradeQuery, error) {
	biz, err := json.Marshal(map[string]string{"out_trade_no": outTradeNo})
	if err != nil {
		return TradeQuery{}, err
	}
	body, err := c.signedPost("alipay.trade.query", string(biz), nil)
	if err != nil {
		return TradeQuery{}, err
	}

	var wrapped struct {
		Response struct {
			Code        string `json:"code"`
			Msg         string `json:"msg"`
			SubMsg      string `json:"sub_msg"`
			OutTradeNo  string `json:"out_trade_no"`
			TradeNo     string `json:"trade_no"`
			TradeStatus string `json:"trade_status"`
			TotalAmount string `json:"total_amount"`
		} `json:"alipay_trade_query_response"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return TradeQuery{}, fmt.Errorf("decode alipay query: %w", err)
	}
	if wrapped.Response.Code != "10000" {
		return TradeQuery{}, fmt.Errorf("alipay query %s: %s %s", wrapped.Response.Code, wrapped.Response.Msg, wrapped.Response.SubMsg)
	}
	return TradeQuery{
		OutTradeNo:  wrapped.Response.OutTradeNo,
		TradeNo:     wrapped.Response.TradeNo,
		TradeStatus: wrapped.Response.TradeStatus,
		TotalAmount: wrapped.Response.TotalAmount,
	}, nil
}

func (c *AlipayClient) VerifyNotify(form url.Values) (Notify, error) {
	params := make(map[string]string, len(form))
	for k, vs := range form {
		if len(vs) == 0 {
			continue
		}
		params[k] = vs[0]
	}
	sign := params["sign"]
	delete(params, "sign")
	delete(params, "sign_type")
	if sign == "" {
		return Notify{}, fmt.Errorf("%w: missing sign", ErrNotifyInvalid)
	}
	if err := verifyRSA2(c.publicKey, params, sign); err != nil {
		return Notify{}, fmt.Errorf("%w: %v", ErrNotifyInvalid, err)
	}
	if params["app_id"] != c.cfg.AppID {
		return Notify{}, fmt.Errorf("%w: app_id mismatch", ErrNotifyInvalid)
	}
	return Notify{
		AppID:       params["app_id"],
		OutTradeNo:  params["out_trade_no"],
		TradeNo:     params["trade_no"],
		TradeStatus: params["trade_status"],
		TotalAmount: params["total_amount"],
	}, nil
}

func (c *AlipayClient) signedPost(method, biz string, extra map[string]string) ([]byte, error) {
	params := c.publicParams(method)
	for k, v := range extra {
		if strings.TrimSpace(v) == "" {
			continue
		}
		params[k] = v
	}
	params["biz_content"] = biz
	sign, err := signRSA2(c.privateKey, params)
	if err != nil {
		return nil, err
	}
	params["sign"] = sign
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	// 沙箱对含中文的 biz_content 会按 URL 上的 charset 解码。charset 只放 POST body
	// 时，网关按 GBK 还原签名串，返回 40002「请确认 charset 参数放在了 URL 查询字符串中」。
	endpoint, err := gatewayWithCharset(c.cfg.Gateway, params["charset"])
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.PostForm(endpoint, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func gatewayWithCharset(gateway, charset string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(gateway))
	if err != nil {
		return "", fmt.Errorf("alipay gateway: %w", err)
	}
	if strings.TrimSpace(charset) == "" {
		charset = "UTF-8"
	}
	q := u.Query()
	q.Set("charset", charset)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *AlipayClient) publicParams(method string) map[string]string {
	return map[string]string{
		"app_id":    c.cfg.AppID,
		"method":    method,
		"format":    "json",
		"charset":   "UTF-8",
		"sign_type": "RSA2",
		"timestamp": c.now().In(shanghai()).Format("2006-01-02 15:04:05"),
		"version":   "1.0",
	}
}

func signContent(params map[string]string, omitSignType bool) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || v == "" {
			continue
		}
		// 请求签名要带 sign_type；异步通知验签要去掉 sign_type。
		if omitSignType && k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}

func signRSA2(key *rsa.PrivateKey, params map[string]string) (string, error) {
	sum := sha256.Sum256([]byte(signContent(params, false)))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func verifyRSA2(key *rsa.PublicKey, params map[string]string, sign string) error {
	raw, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(signContent(params, true)))
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], raw)
}

func qrImageDataURL(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}
	png, err := qrcode.Encode(content, qrcode.Medium, 240)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, err := decodePEM(raw, "PRIVATE KEY")
	if err != nil {
		return nil, err
	}
	if key, err := x509.ParsePKCS8PrivateKey(block); err == nil {
		priv, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA private key")
		}
		return priv, nil
	}
	return x509.ParsePKCS1PrivateKey(block)
}

func parseRSAPublicKey(raw string) (*rsa.PublicKey, error) {
	block, err := decodePEM(raw, "PUBLIC KEY")
	if err != nil {
		return nil, err
	}
	if key, err := x509.ParsePKIXPublicKey(block); err == nil {
		pub, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA public key")
		}
		return pub, nil
	}
	return x509.ParsePKCS1PublicKey(block)
}

func decodePEM(raw, typ string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, `\n`, "\n")
	if !strings.Contains(s, "BEGIN") {
		var b strings.Builder
		b.WriteString("-----BEGIN ")
		b.WriteString(typ)
		b.WriteString("-----\n")
		compact := strings.ReplaceAll(strings.ReplaceAll(s, "\n", ""), "\r", "")
		for compact != "" {
			n := 64
			if n > len(compact) {
				n = len(compact)
			}
			b.WriteString(compact[:n])
			b.WriteByte('\n')
			compact = compact[n:]
		}
		b.WriteString("-----END ")
		b.WriteString(typ)
		b.WriteString("-----\n")
		s = b.String()
	}
	block, _ := pem.Decode([]byte(s))
	if block == nil {
		return nil, fmt.Errorf("invalid pem")
	}
	return block.Bytes, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func paidStatus(status string) bool {
	return status == tradeSuccess || status == tradeFinished
}
