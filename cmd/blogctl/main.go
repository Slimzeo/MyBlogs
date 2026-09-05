package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

var cliVersion = "0.1.0"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "blogctl:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	case "version", "--version":
		fmt.Fprintln(stdout, "blogctl", cliVersion)
		return nil
	case "auth":
		return runAuth(args[1:], stdout, stderr)
	case "article":
		return runArticle(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("未知命令 %q；执行 blogctl help 查看用法", args[0])
	}
}

func runAuth(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("请指定 auth login、auth status 或 auth logout")
	}
	switch args[0] {
	case "login":
		return runAuthLogin(args[1:], stdout, stderr)
	case "status":
		return runAuthStatus(args[1:], stdout, stderr)
	case "logout":
		return runAuthLogout(args[1:], stdout)
	default:
		return fmt.Errorf("未知 auth 命令 %q", args[0])
	}
}

func runAuthLogin(args []string, stdout, stderr io.Writer) error {
	current, err := loadConfig()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("auth login", flag.ContinueOnError)
	flags.SetOutput(stderr)
	server := flags.String("server", current.Server, "博客服务地址，例如 https://www.hypn0s.cloud")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("auth login 不接受位置参数")
	}
	if strings.TrimSpace(*server) == "" {
		return errors.New("请通过 --server 或 BLOGCTL_SERVER 指定博客地址")
	}
	token := strings.TrimSpace(os.Getenv("BLOGCTL_TOKEN"))
	if token == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("非交互环境请通过 BLOGCTL_TOKEN 提供密钥")
		}
		fmt.Fprint(stderr, "Agent Token（输入不会显示）: ")
		value, readErr := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(stderr)
		if readErr != nil {
			return fmt.Errorf("读取 Token: %w", readErr)
		}
		token = strings.TrimSpace(string(value))
	}
	client, err := newAPIClient(*server, token)
	if err != nil {
		return err
	}
	info, err := client.checkAuth()
	if err != nil {
		return fmt.Errorf("认证失败: %w", err)
	}
	if err := saveConfig(cliConfig{Server: client.server, Token: token}); err != nil {
		return err
	}
	path, _ := configPath()
	fmt.Fprintf(stdout, "认证成功：%s（%s），凭据已保存到 %s\n", info.Name, info.Scope, path)
	return nil
}

func runAuthStatus(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("auth status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "输出 JSON")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("auth status 不接受位置参数")
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	client, err := newAPIClient(config.Server, config.Token)
	if err != nil {
		return err
	}
	info, err := client.checkAuth()
	if err != nil {
		return fmt.Errorf("认证不可用: %w", err)
	}
	if *jsonOutput {
		return writeJSON(stdout, map[string]any{"authenticated": true, "server": client.server, "name": info.Name, "scope": info.Scope, "expiresAt": info.ExpiresAt})
	}
	fmt.Fprintf(stdout, "已认证到 %s\n名称：%s\n权限：%s\n到期：%s\n", client.server, info.Name, info.Scope, formatUnixTime(info.ExpiresAt))
	return nil
}

func runAuthLogout(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("auth logout 不接受参数")
	}
	if err := removeConfig(); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "已删除本机保存的 blogctl 凭据。")
	if strings.TrimSpace(os.Getenv("BLOGCTL_TOKEN")) != "" {
		fmt.Fprintln(stdout, "注意：当前进程环境仍设置了 BLOGCTL_TOKEN。")
	}
	return nil
}

func runArticle(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("请指定 article validate 或 article import")
	}
	switch args[0] {
	case "validate":
		return runArticleCommand(false, args[1:], stdout, stderr)
	case "import":
		return runArticleCommand(true, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("未知 article 命令 %q", args[0])
	}
}

func runArticleCommand(importArticle bool, args []string, stdout, stderr io.Writer) error {
	name := "article validate"
	if importArticle {
		name = "article import"
	}
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	title := flags.String("title", "", "覆盖文章标题")
	slug := flags.String("slug", "", "覆盖文章 slug")
	tags := flags.String("tags", "", "覆盖标签，英文逗号分隔")
	categories := flags.String("categories", "", "覆盖分类，英文逗号分隔")
	displayTime := flags.String("display-time", "", "覆盖显示时间（YYYY-MM-DDTHH:MM 或 RFC3339）")
	server := flags.String("server", "", "仅本次使用的博客地址")
	jsonOutput := flags.Bool("json", false, "输出 JSON，适合本地 Agent")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("用法：blogctl %s [flags] <article.md|article.html|archive.zip>", name)
	}
	upload, err := prepareArticle(flags.Arg(0), articleOverrides{
		Title: *title, Slug: *slug, Tags: *tags, Categories: *categories, DisplayTime: *displayTime,
	})
	if err != nil {
		return err
	}
	if !importArticle {
		if *jsonOutput {
			return writeJSON(stdout, map[string]any{
				"valid": true, "file": upload.Filename, "format": upload.Format,
				"bytes": len(upload.Data), "assets": upload.Assets, "metadata": upload.Metadata,
			})
		}
		fmt.Fprintf(stdout, "校验通过：%s（%s，%d bytes，%d 个本地资源）\n", upload.Filename, upload.Format, len(upload.Data), len(upload.Assets))
		return nil
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	if strings.TrimSpace(*server) != "" {
		config.Server = *server
	}
	client, err := newAPIClient(config.Server, config.Token)
	if err != nil {
		return err
	}
	result, err := client.importArticle(upload)
	if err != nil {
		return err
	}
	if *jsonOutput {
		return writeJSON(stdout, map[string]any{
			"success": true, "server": client.server, "article": result,
		})
	}
	previewURL, _ := url.JoinPath(client.server, result.PreviewPath)
	fmt.Fprintf(stdout, "已导入草稿 #%d：%s\n预览：%s\n", result.ID, result.Title, previewURL)
	if result.Replayed {
		fmt.Fprintln(stdout, "该请求已处理过，本次返回原导入结果。")
	}
	return nil
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func formatUnixTime(unixTime int) string {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	return time.Unix(int64(unixTime), 0).In(location).Format("2006-01-02 15:04 MST")
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, `blogctl - MyBlogs 本地文章导入工具

用法：
  blogctl auth login --server https://blog.example.com
  blogctl auth status [--json]
  blogctl auth logout
  blogctl article validate [flags] article.md
  blogctl article import [flags] article.md

文章 flags：
  --title --slug --tags --categories --display-time --server --json

自动化环境可设置 BLOGCTL_SERVER、BLOGCTL_TOKEN 和 BLOGCTL_CONFIG。
CLI v1 只创建草稿，不提供发布、删除或系统设置权限。`)
}
