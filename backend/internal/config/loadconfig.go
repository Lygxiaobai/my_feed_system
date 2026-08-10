package config

import (
	"fmt"
	"os"

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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// 用环境变量展开配置内容，便于把 JWT 密钥等敏感项通过环境注入而不写进 Git。
	data = []byte(os.Expand(string(data), os.Getenv))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config file: %w", err)
	}

	return &cfg, nil
}
