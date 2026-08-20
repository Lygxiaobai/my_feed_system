package account

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Mailer 发送登录验证码。实现不得把授权码写入日志。
type Mailer interface {
	Configured() bool
	SendCode(to string, code string, ttlMinutes int) error
}

// SMTPMailer 走通用 SMTP。默认按网易 163 的 465/SSL。
type SMTPMailer struct {
	Host     string
	Port     int
	TLS      string
	User     string
	Password string
	From     string
}

func (m *SMTPMailer) Configured() bool {
	return m != nil && strings.TrimSpace(m.User) != "" && strings.TrimSpace(m.Password) != ""
}

func (m *SMTPMailer) fromAddr() string {
	if from := strings.TrimSpace(m.From); from != "" {
		return from
	}
	return strings.TrimSpace(m.User)
}

func (m *SMTPMailer) SendCode(to string, code string, ttlMinutes int) error {
	if strings.TrimSpace(m.Host) == "" {
		m.Host = "smtp.163.com"
	}
	if m.Port == 0 {
		m.Port = 465
	}
	from := m.fromAddr()
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: 登录验证码",
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		fmt.Sprintf("您的验证码是 %s，%d 分钟内有效。", code, ttlMinutes),
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)
	auth := smtp.PlainAuth("", m.User, m.Password, m.Host)
	if strings.EqualFold(strings.TrimSpace(m.TLS), "starttls") {
		return sendStartTLS(addr, m.Host, auth, from, []string{to}, []byte(msg))
	}
	return sendImplicitTLS(addr, m.Host, auth, from, []string{to}, []byte(msg))
}

func sendImplicitTLS(addr string, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("dial smtp tls: %w", err)
	}
	defer conn.Close()
	return smtpOnConn(conn, host, auth, from, to, msg)
}

func sendStartTLS(addr string, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()
	if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}
	return smtpSend(client, auth, from, to, msg)
}

func smtpOnConn(conn net.Conn, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()
	return smtpSend(client, auth, from, to, msg)
}

func smtpSend(client *smtp.Client, auth smtp.Auth, from string, to []string, msg []byte) error {
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("smtp rcpt: %w", err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return client.Quit()
}
