package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `
server:
  port: ${SERVER_PORT:-8081}
database:
  host: ${MYSQL_HOST:-127.0.0.1}
  password: ${MYSQL_PASSWORD}
  dbname: ${MYSQL_DATABASE:-my_feed_system}
rabbitmq:
  password: ${RABBITMQ_PASSWORD}
jwt:
  secret: ${JWT_SECRET}
`

func write(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(p, []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func clearSecrets(t *testing.T) {
	t.Helper()
	for _, k := range []string{"MYSQL_PASSWORD", "RABBITMQ_PASSWORD", "JWT_SECRET", "SERVER_PORT", "MYSQL_HOST"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

// 缺失必填变量必须一次性列出全部，而不是静默展开为空。
func TestMissingRequiredVarsFailLoudly(t *testing.T) {
	clearSecrets(t)
	_, err := Load(write(t))
	if err == nil {
		t.Fatal("期望启动失败，实际成功了——这正是原先的漏洞")
	}
	for _, want := range []string{"MYSQL_PASSWORD", "RABBITMQ_PASSWORD", "JWT_SECRET"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息未提到缺失的 %s: %v", want, err)
		}
	}
}

// 过短的 JWT 密钥必须被拒绝（含已泄漏的旧默认值 my-feed-system-secret）。
func TestWeakJWTSecretRejected(t *testing.T) {
	clearSecrets(t)
	t.Setenv("MYSQL_PASSWORD", "x")
	t.Setenv("RABBITMQ_PASSWORD", "x")
	t.Setenv("JWT_SECRET", "my-feed-system-secret")
	if _, err := Load(write(t)); err == nil {
		t.Fatal("期望拒绝弱密钥")
	}
}

// 默认值语法生效，且显式设置的环境变量优先。
func TestDefaultsAndOverride(t *testing.T) {
	clearSecrets(t)
	t.Setenv("MYSQL_PASSWORD", "x")
	t.Setenv("RABBITMQ_PASSWORD", "x")
	t.Setenv("JWT_SECRET", strings.Repeat("k", 40))

	cfg, err := Load(write(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8081 {
		t.Errorf("默认值未生效: port=%d", cfg.Server.Port)
	}
	if cfg.Database.Host != "127.0.0.1" {
		t.Errorf("默认值未生效: host=%s", cfg.Database.Host)
	}

	t.Setenv("SERVER_PORT", "9999")
	cfg2, err := Load(write(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Server.Port != 9999 {
		t.Errorf("环境变量未覆盖默认值: port=%d", cfg2.Server.Port)
	}
	if cfg.Audit.Enabled {
		t.Fatal("未配置 audit.enabled 时必须默认关闭")
	}
}

func TestAuditEnabledFromYAML(t *testing.T) {
	clearSecrets(t)
	t.Setenv("MYSQL_PASSWORD", "x")
	t.Setenv("RABBITMQ_PASSWORD", "x")
	t.Setenv("JWT_SECRET", strings.Repeat("k", 40))

	p := filepath.Join(t.TempDir(), "cfg.yaml")
	content := sample + "audit:\n  enabled: ${AUDIT_ENABLED:-false}\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Audit.Enabled {
		t.Fatal("AUDIT_ENABLED 未设置时必须默认关闭")
	}

	t.Setenv("AUDIT_ENABLED", "true")
	cfg2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg2.Audit.Enabled {
		t.Fatal("AUDIT_ENABLED=true 时应开启审核")
	}
}

// .env 文件能补齐变量，但不覆盖已存在的环境变量。
func TestDotEnvDoesNotOverrideExistingEnv(t *testing.T) {
	clearSecrets(t)
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	content := "MYSQL_PASSWORD=from-dotenv\n# 注释行\nexport JWT_SECRET=" + strings.Repeat("d", 40) + "\nRABBITMQ_PASSWORD=\"quoted-value\"\n"
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MYSQL_PASSWORD", "from-real-env")
	loadDotEnv(envFile)

	if got := os.Getenv("MYSQL_PASSWORD"); got != "from-real-env" {
		t.Errorf("已有环境变量被 .env 覆盖了: %s", got)
	}
	if got := os.Getenv("RABBITMQ_PASSWORD"); got != "quoted-value" {
		t.Errorf("引号未被正确剥离: %q", got)
	}
	if got := os.Getenv("JWT_SECRET"); got != strings.Repeat("d", 40) {
		t.Errorf("export 前缀未被处理: %q", got)
	}
}
