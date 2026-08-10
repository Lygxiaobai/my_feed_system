package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	RabbitMQ RabbitMQConfig `yaml:"rabbitmq"`
	JWT      JWTConfig      `yaml:"jwt"`
	Upload   UploadConfig   `yaml:"upload"`
	Pprof    PprofConfig    `yaml:"pprof"`
	Log      LogConfig      `yaml:"log"`
	Audit    AuditConfig    `yaml:"audit"`
}

// AuditConfig 控制内容审核。
//
// 词库文件本身属于敏感资产（会被用来反推规则并试探绕过），
// 与密钥同等对待：不进版本库，通过路径引用。
type AuditConfig struct {
	// BlockWordFile 命中即拒绝的词库路径。
	BlockWordFile string `yaml:"block_word_file"`
	// ReviewWordFile 命中转人工的词库路径。
	ReviewWordFile string `yaml:"review_word_file"`
	// MediaPolicy 在未接入媒体审核能力时的处置方式：review（默认）或 pass。
	// 生产环境应保持 review——没有能力审核就不能假装审过了。
	MediaPolicy string `yaml:"media_policy"`
	// ReviewerAccountIDs 是有人工复审权限的账号。
	// 当前系统只有「是/不是审核员」一个区分，用白名单而不是引入一套 RBAC。
	ReviewerAccountIDs []uint64 `yaml:"reviewer_account_ids"`
}

// LogConfig 控制日志输出。
// Level 取 debug / info / warn / error，生产环境不应使用 debug；
// Format 取 json / text，线上用 json 以便被采集系统直接解析。
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type RabbitMQConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// VHost 为空时默认使用 "/"。
	VHost string `yaml:"vhost"`
	// PrefetchCount 控制每个消费者可同时持有的未 ACK 消息数量。
	PrefetchCount int `yaml:"prefetch_count"`
	// ConsumerTag 用于在 RabbitMQ 控制台区分消费者实例。
	ConsumerTag string `yaml:"consumer_tag"`
}

type JWTConfig struct {
	Secret string `yaml:"secret"`
}

type UploadConfig struct {
	Dir           string `yaml:"dir"`
	MaxVideoBytes int64  `yaml:"max_video_bytes"`
}

type PprofConfig struct {
	API    PprofServerConfig `yaml:"api"`
	Worker PprofServerConfig `yaml:"worker"`
}

type PprofServerConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
}

func Load(path string) (*Config, error) {
	// 本地开发从工作目录下的 .env 补齐环境变量（该文件已被 .gitignore 排除）。
	// 容器部署不带 .env，配置全部由编排层注入，两种场景走同一套代码。
	loadDotEnv(".env")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	expanded, missing := expandPlaceholders(string(data))
	if len(missing) > 0 {
		// 一次性列出全部缺失项，避免改一个报一个。
		return nil, fmt.Errorf(
			"配置缺少必需的环境变量: %s（本地开发可复制 backend/.env.example 为 backend/.env 并填写）",
			strings.Join(missing, ", "),
		)
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config file: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// placeholderPattern 匹配 ${VAR} 与 ${VAR:-default} 两种写法。
var placeholderPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

// expandPlaceholders 展开配置中的环境变量占位符，并返回缺失的必填变量名。
//
// 不使用 os.Expand 的原因：它把未设置的变量静默展开为空字符串，
// 且不支持默认值语法。对密钥类配置而言这非常危险——缺少 JWT_SECRET
// 会得到一个空密钥，服务照常启动，但任何人都能伪造 token。
// 这里区分两种语义：
//
//	${VAR}            必填，未设置时收集进 missing 并让启动失败
//	${VAR:-default}   选填，未设置时使用默认值（只用于非敏感项）
func expandPlaceholders(raw string) (string, []string) {
	var missing []string
	seen := make(map[string]struct{})

	out := placeholderPattern.ReplaceAllStringFunc(raw, func(match string) string {
		groups := placeholderPattern.FindStringSubmatch(match)
		name := groups[1]
		hasDefault := strings.Contains(match, ":-")

		if value, ok := os.LookupEnv(name); ok && value != "" {
			return value
		}
		if hasDefault {
			return groups[2]
		}
		if _, dup := seen[name]; !dup {
			seen[name] = struct{}{}
			missing = append(missing, name)
		}
		return ""
	})

	return out, missing
}

// minJWTSecretLength 是 JWT 密钥的最小长度。
// HS256 的安全性完全取决于密钥强度，过短的密钥可被离线暴力破解。
const minJWTSecretLength = 32

// validate 在配置解析完成后做安全性兜底校验。
// 这类问题如果放到运行期才暴露，往往已经签发了一批不可信的 token。
func (c *Config) validate() error {
	secret := strings.TrimSpace(c.JWT.Secret)
	if secret == "" {
		return errors.New("jwt.secret 为空：请设置 JWT_SECRET 环境变量")
	}
	if len(secret) < minJWTSecretLength {
		return fmt.Errorf(
			"jwt.secret 长度不足 %d 位（当前 %d 位）：请用 openssl rand -base64 48 生成后设置 JWT_SECRET",
			minJWTSecretLength, len(secret),
		)
	}
	if c.Database.DBName == "" {
		return errors.New("database.dbname 为空：请检查配置文件")
	}
	return nil
}
