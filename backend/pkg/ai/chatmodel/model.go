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

package chatmodel

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	arkmodel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

type CreateChatModelOption func(o *option)

type ModelType string

const (
	ARK_MODEL    ModelType = "ark"
	OPENAI_MODEL ModelType = "openai"
	DEEPSEEK     ModelType = "deepseek"
)

type ModelConfig struct {
	ModelType ModelType
	ModelName string
	ApiKey    string
	BaseURL   string
	Thinking  bool
}

func NewModel(ctx context.Context, config ModelConfig, opts ...CreateChatModelOption) (cm model.ToolCallingChatModel, err error) {
	o := &option{}
	for _, opt := range opts {
		opt(o)
	}
	modelType := config.ModelType
	modelName := config.ModelName
	apiKey := config.ApiKey
	baseURL := config.BaseURL

	switch modelType {
	case ARK_MODEL:
		conf := &ark.ChatModelConfig{
			APIKey:  apiKey,
			BaseURL: baseURL,
			// Region:      os.Getenv("ARK_REGION"),
			Model:       modelName,
			MaxTokens:   o.MaxTokens,
			Temperature: o.Temperature,
			TopP:        o.TopP,
		}
		if o.DisableThinking != nil && *o.DisableThinking {
			conf.Thinking = &arkmodel.Thinking{
				Type: arkmodel.ThinkingTypeDisabled,
			}
		}
		if o.JsonSchema != nil {
			conf.ResponseFormat = &ark.ResponseFormat{
				Type: arkmodel.ResponseFormatJSONSchema,
				JSONSchema: &arkmodel.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:        o.JsonSchema.Name,
					Description: o.JsonSchema.Description,
					Schema:      o.JsonSchema.JSONSchema,
					Strict:      o.JsonSchema.Strict,
				},
			}
		}
		cm, err = ark.NewChatModel(ctx, conf)

	case OPENAI_MODEL:
		conf := &openai.ChatModelConfig{
			APIKey: apiKey,
			ByAzure: func() bool {
				return os.Getenv("OPENAI_BY_AZURE") == "true"
			}(),
			BaseURL:         baseURL,
			Model:           modelName,
			MaxTokens:       o.MaxTokens,
			Temperature:     o.Temperature,
			TopP:            o.TopP,
			ReasoningEffort: openai.ReasoningEffortLevelMedium,
		}
		if config.Thinking {
			conf.ExtraFields = map[string]any{
				"thinking": map[string]any{"type": "enabled"},
			}
		}
		if o.JsonSchema != nil {
			conf.ResponseFormat = &openai.ChatCompletionResponseFormat{
				Type:       openai.ChatCompletionResponseFormatTypeJSONSchema,
				JSONSchema: o.JsonSchema,
			}
		}
		cm, err = openai.NewChatModel(ctx, conf)
	case DEEPSEEK:
		modelName = "deepseek-chat"
		if config.Thinking {
			modelName = "deepseek-reasoner"
		}
		conf := &deepseek.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: baseURL,
			// Thinking: &deepseekModel.Thinking{
			// 	Type: deepseekModel.ThinkingTypeEnabled,
			// },
		}
		if o.JsonSchema != nil {
			conf.ResponseFormatType = deepseek.ResponseFormatTypeJSONObject
		}
		cm, err = deepseek.NewChatModel(context.Background(), conf)
	}
	if err != nil {
		return nil, err
	}
	if cm == nil {
		return nil, fmt.Errorf("no model config")
	}

	return cm, nil
}

type option struct {
	MaxTokens       *int
	Temperature     *float32
	TopP            *float32
	DisableThinking *bool
	JsonSchema      *openai.ChatCompletionResponseFormatJSONSchema
}

func WithMaxTokens(maxTokens int) CreateChatModelOption {
	return func(o *option) {
		o.MaxTokens = &maxTokens
	}
}

func WithTemperature(temp float32) CreateChatModelOption {
	return func(o *option) {
		o.Temperature = &temp
	}
}

func WithTopP(topP float32) CreateChatModelOption {
	return func(o *option) {
		o.TopP = &topP
	}
}

func WithDisableThinking(disable bool) CreateChatModelOption {
	return func(o *option) {
		o.DisableThinking = &disable
	}
}

func WithResponseFormatJsonSchema(schema *openai.ChatCompletionResponseFormatJSONSchema) CreateChatModelOption {
	return func(o *option) {
		o.JsonSchema = schema
	}
}
