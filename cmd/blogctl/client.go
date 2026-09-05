package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type apiClient struct {
	server string
	token  string
	http   *http.Client
}

type authInfo struct {
	Name      string `json:"name"`
	Scope     string `json:"scope"`
	ExpiresAt int    `json:"expiresAt"`
}

type articleImportResponse struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	EditPath    string `json:"editPath"`
	PreviewPath string `json:"previewPath"`
	Replayed    bool   `json:"replayed"`
}

type apiEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func newAPIClient(server, token string) (*apiClient, error) {
	normalized, err := normalizeServer(server)
	if err != nil {
		return nil, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("没有可用的 Token；请先执行 blogctl auth login，或设置 BLOGCTL_TOKEN")
	}
	return &apiClient{
		server: normalized,
		token:  token,
		http: &http.Client{
			Timeout: 2 * time.Minute,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (client *apiClient) checkAuth() (authInfo, error) {
	var info authInfo
	request, err := client.newRequest(http.MethodGet, "/api/v1/auth", nil)
	if err != nil {
		return info, err
	}
	if err := client.do(request, &info); err != nil {
		return info, err
	}
	return info, nil
}

func (client *apiClient) importArticle(upload articleUpload) (articleImportResponse, error) {
	var result articleImportResponse
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("archive", upload.Filename)
	if err != nil {
		return result, err
	}
	if _, err := file.Write(upload.Data); err != nil {
		return result, err
	}
	fields := map[string]string{
		"title":        upload.Metadata.Title,
		"slug":         upload.Metadata.Slug,
		"tags":         upload.Metadata.Tags,
		"categories":   upload.Metadata.Categories,
		"display_time": upload.Metadata.DisplayTime,
	}
	for name, value := range fields {
		if value == "" {
			continue
		}
		if err := writer.WriteField(name, value); err != nil {
			return result, err
		}
	}
	if err := writer.Close(); err != nil {
		return result, err
	}
	request, err := client.newRequest(http.MethodPost, "/api/v1/articles/import", &body)
	if err != nil {
		return result, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Idempotency-Key", articleIdempotencyKey(upload))
	if err := client.do(request, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (client *apiClient) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	requestURL, err := url.JoinPath(client.server, path)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+client.token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "blogctl/"+cliVersion)
	return request, nil
}

func (client *apiClient) do(request *http.Request, output any) error {
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("请求 %s 失败: %w", client.server, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("读取服务端响应: %w", err)
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("服务端返回了非预期响应 (HTTP %d)", response.StatusCode)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !envelope.Success {
		message := strings.TrimSpace(envelope.Error.Message)
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		if envelope.Error.Code != "" {
			return fmt.Errorf("%s: %s", envelope.Error.Code, message)
		}
		return errors.New(message)
	}
	if output == nil || len(envelope.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, output); err != nil {
		return fmt.Errorf("解析服务端数据: %w", err)
	}
	return nil
}

func articleIdempotencyKey(upload articleUpload) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(upload.Filename))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(upload.Data)
	for _, value := range []string{
		upload.Metadata.Title,
		upload.Metadata.Slug,
		upload.Metadata.Tags,
		upload.Metadata.Categories,
		upload.Metadata.DisplayTime,
	} {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(value))
	}
	return hex.EncodeToString(hash.Sum(nil))
}
