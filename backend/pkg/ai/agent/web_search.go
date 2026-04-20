/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agent

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/example/aichat/backend/pkg/ai"
	"github.com/example/aichat/backend/pkg/ai/chatmodel"
)

func NewWebSearchAgent(ctx context.Context, config *ai.AgentConfig) (adk.Agent, error) {
	cm, err := chatmodel.NewModel(ctx, chatmodel.ModelConfig{
		ModelType: chatmodel.ModelType(config.ModelConfig.ModelType),
		ModelName: config.ModelConfig.ModelName,
		ApiKey:    config.ModelConfig.APIKey,
		BaseURL:   config.ModelConfig.BaseURL,
		Thinking:  config.ModelConfig.Thinking,
	},
		chatmodel.WithMaxTokens(config.ModelConfig.MaxTokens),
		chatmodel.WithTemperature(config.ModelConfig.Temperature),
		chatmodel.WithTopP(config.ModelConfig.TopP),
		chatmodel.WithDisableThinking(!config.ModelConfig.Thinking),
	)
	if err != nil {
		return nil, err
	}

	searchTool, err := duckduckgo.NewTextSearchTool(ctx, &duckduckgo.Config{
		ToolName:   "duckduckgo_search",
		ToolDesc:   "search for information by duckduckgo",
		Timeout:    5 * time.Minute,
		Region:     duckduckgo.RegionWT, // The geographical region for results.
		MaxResults: 3,                   // Limit the number of results returned.
		HTTPClient: newDuckDuckGoHTTPClient(),
	})
	if err != nil {
		return nil, err
	}

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "WebSearchAgent",
		Description: "WebSearchAgent 支持联网搜索",
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{searchTool},
			},
		},
		MaxIterations: 30,
	})
}

func newDuckDuckGoHTTPClient() *http.Client {
	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	baseTransport.Proxy = proxyFromEnv("DDG_PROXY")
	baseTransport.MaxIdleConnsPerHost = 10
	baseTransport.IdleConnTimeout = 90 * time.Second

	return &http.Client{
		Timeout: 5 * time.Minute,
		Transport: &retryTransport{
			base:       baseTransport,
			maxRetries: 4,
			backoff:    3 * time.Second,
			addHeaders: http.Header{
				"Accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
				"Accept-Language": {"en-US,en;q=0.9"},
				"DNT":             {"1"},
			},
			forceHeaders: http.Header{
				"User-Agent": {"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"},
			},
		},
	}
}

type retryTransport struct {
	base         http.RoundTripper
	maxRetries   int
	backoff      time.Duration
	addHeaders   http.Header
	forceHeaders http.Header
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}

	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		r := req.Clone(req.Context())
		if bodyBytes != nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		for key, values := range t.addHeaders {
			if len(values) == 0 {
				continue
			}
			if r.Header.Get(key) == "" {
				for _, v := range values {
					r.Header.Add(key, v)
				}
			}
		}
		for key, values := range t.forceHeaders {
			if len(values) == 0 {
				continue
			}
			r.Header.Del(key)
			for _, v := range values {
				r.Header.Add(key, v)
			}
		}

		resp, err := base.RoundTrip(r)
		if err == nil && resp != nil && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable {
			return resp, nil
		}

		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if attempt == t.maxRetries {
			return resp, err
		}
		delay := t.retryDelay(resp, attempt)
		select {
		case <-time.After(delay):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	return nil, nil
}

func (t *retryTransport) retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
			if when, err := http.ParseTime(retryAfter); err == nil {
				delay := time.Until(when)
				if delay > 0 {
					return delay
				}
			}
		}
	}
	return t.backoff * time.Duration(attempt+1)
}

func proxyFromEnv(primaryKey string) func(*http.Request) (*url.URL, error) {
	if value, ok := lookupEnvAny(primaryKey); ok {
		switch value {
		case "direct", "DIRECT", "none", "NONE", "0", "false", "FALSE":
			return nil
		default:
			return func(_ *http.Request) (*url.URL, error) {
				return url.Parse(value)
			}
		}
	}
	return http.ProxyFromEnvironment
}

func lookupEnvAny(key string) (string, bool) {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val, true
	}
	return "", false
}
