package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client 通用HTTP客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
	authToken  string
	maxRetries int
}

// NewClient 创建新的HTTP客户端
func NewClient(cfg ClientConfig) *Client {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 1 // 默认重试3次
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL:    cfg.BaseURL,
		authToken:  cfg.AuthToken,
		maxRetries: cfg.MaxRetries,
	}
}

// DoRequest 执行HTTP请求
func (c *Client) DoRequest(ctx context.Context, opts RequestOptions) error {
	return c.doRequestOnce(ctx, opts)
}

// doRequestOnce 执行单次HTTP请求
func (c *Client) doRequestOnce(ctx context.Context, opts RequestOptions) error {
	// 构建URL
	reqURL := c.baseURL + opts.Path

	// 添加查询参数
	if len(opts.QueryParams) > 0 {
		params := url.Values{}
		for k, v := range opts.QueryParams {
			params.Set(k, v)
		}
		reqURL += "?" + params.Encode()
	}

	// 构建请求体
	var bodyReader io.Reader
	if opts.Body != nil {
		data, err := json.Marshal(opts.Body)
		if err != nil {
			return fmt.Errorf("序列化请求体失败: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, opts.Method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置默认请求头
	req.Header.Set("Content-Type", "application/json")

	// 设置认证Token
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	// 设置自定义请求头
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查HTTP状态码
	/*if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}*/

	// 解析响应体
	if opts.Response != nil {
		if err := json.Unmarshal(body, opts.Response); err != nil {
			return fmt.Errorf("解析响应失败: %w, 响应体: %s", err, string(body))
		}
	}

	return nil
}

// DoRequestRaw 执行HTTP请求并返回原始响应（不自动解析JSON）
func (c *Client) DoRequestRaw(ctx context.Context, opts RequestOptions) ([]byte, error) {
	var responseBody []byte
	var err error

	operation := func() error {
		// 构建URL
		reqURL := c.baseURL + opts.Path

		// 添加查询参数
		if len(opts.QueryParams) > 0 {
			params := url.Values{}
			for k, v := range opts.QueryParams {
				params.Set(k, v)
			}
			reqURL += "?" + params.Encode()
		}

		// 构建请求体
		var bodyReader io.Reader
		if opts.Body != nil {
			data, marshalErr := json.Marshal(opts.Body)
			if marshalErr != nil {
				return fmt.Errorf("序列化请求体失败: %w", marshalErr)
			}
			bodyReader = bytes.NewReader(data)
		}

		// 创建HTTP请求
		req, createErr := http.NewRequestWithContext(ctx, opts.Method, reqURL, bodyReader)
		if createErr != nil {
			return fmt.Errorf("创建请求失败: %w", createErr)
		}

		// 设置默认请求头
		req.Header.Set("Content-Type", "application/json")

		// 设置认证Token
		if c.authToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.authToken)
		}

		// 设置自定义请求头
		for k, v := range opts.Headers {
			req.Header.Set(k, v)
		}

		// 发送请求
		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return fmt.Errorf("请求失败: %w", doErr)
		}
		defer resp.Body.Close()

		// 读取响应体
		responseBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("读取响应失败: %w", err)
		}

		// 检查HTTP状态码
		if resp.StatusCode >= 400 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(responseBody))
		}

		return nil
	}

	if err := operation(); err != nil {
		return nil, err
	}

	return responseBody, nil
}
