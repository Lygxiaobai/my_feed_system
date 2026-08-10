package config

import (
	"bufio"
	"os"
	"strings"
)

// loadDotEnv 把 .env 文件中的键值补进环境变量，供本地开发使用。
//
// 约定与 docker compose 的 .env 一致，开发者只需 `cp backend/.env.example backend/.env`
// 就能跑起来，不必每开一个终端就 export 一遍。
//
// 三条原则：
//  1. 文件不存在时静默返回——容器部署本来就不该有 .env，配置由编排层注入；
//  2. 已存在的环境变量优先，绝不覆盖，保证显式 export 和 CI 注入的值不被文件顶掉；
//  3. 只做最朴素的解析，不支持变量插值等高级语法，避免这里变成第二套配置系统。
func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseDotEnvLine(scanner.Text())
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

// parseDotEnvLine 解析单行 KEY=VALUE，返回是否为有效配置行。
func parseDotEnvLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	// 兼容 shell 风格的 `export KEY=VALUE`。
	trimmed = strings.TrimPrefix(trimmed, "export ")

	key, value, found := strings.Cut(trimmed, "=")
	if !found {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", false
	}

	value = strings.TrimSpace(value)
	// 去掉成对的引号，允许值里含空格。
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	return key, value, true
}
