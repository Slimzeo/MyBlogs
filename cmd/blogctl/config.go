package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type cliConfig struct {
	Server string `json:"server"`
	Token  string `json:"token"`
}

func loadConfig() (cliConfig, error) {
	var config cliConfig
	path, err := configPath()
	if err != nil {
		return config, err
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return config, fmt.Errorf("读取配置: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return config, fmt.Errorf("解析配置 %s: %w", path, err)
		}
	}
	if value := strings.TrimSpace(os.Getenv("BLOGCTL_SERVER")); value != "" {
		config.Server = value
	}
	if value := strings.TrimSpace(os.Getenv("BLOGCTL_TOKEN")); value != "" {
		config.Token = value
	}
	if strings.TrimSpace(config.Server) != "" {
		config.Server, err = normalizeServer(config.Server)
		if err != nil {
			return config, err
		}
	}
	return config, nil
}

func saveConfig(config cliConfig) error {
	server, err := normalizeServer(config.Server)
	if err != nil {
		return err
	}
	config.Server = server
	config.Token = strings.TrimSpace(config.Token)
	if config.Token == "" {
		return errors.New("Token 不能为空")
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("创建配置目录: %w", err)
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".credentials-*")
	if err != nil {
		return fmt.Errorf("创建临时配置: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("保存配置: %w", err)
	}
	return os.Chmod(path, 0o600)
}

func removeConfig() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除配置: %w", err)
	}
	return nil
}

func configPath() (string, error) {
	if value := strings.TrimSpace(os.Getenv("BLOGCTL_CONFIG")); value != "" {
		return filepath.Abs(value)
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("定位用户配置目录: %w", err)
	}
	return filepath.Join(directory, "myblogs", "credentials.json"), nil
}

func normalizeServer(value string) (string, error) {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("Server 应为完整地址，例如 https://www.hypn0s.cloud")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", errors.New("Server 地址不能包含路径、查询参数或片段")
	}
	hostname := parsed.Hostname()
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(hostname)) {
		return "", errors.New("Server 必须使用 HTTPS；仅 localhost/回环地址允许 HTTP")
	}
	return strings.TrimSuffix(parsed.Scheme+"://"+parsed.Host, "/"), nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
