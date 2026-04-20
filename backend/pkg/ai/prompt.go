package ai

import (
	"bytes"
	"text/template"
)

// PromptManager 管理提示词的渲染和处理
type PromptManager struct {
	config PromptConfig
}

// NewPromptManager 创建一个新的提示词管理器
func NewPromptManager(config PromptConfig) *PromptManager {
	return &PromptManager{
		config: config,
	}
}

// RenderSystemPrompt 渲染系统提示词
func (pm *PromptManager) RenderSystemPrompt() string {
	return pm.config.SystemPrompt
}

// RenderUserPrompt 渲染用户提示词，替换变量
func (pm *PromptManager) RenderUserPrompt() (string, error) {
	tmpl, err := template.New("user_prompt").Parse(pm.config.UserPrompt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, pm.config.Variables); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RenderUserPromptWithVariables 使用指定变量渲染用户提示词
func (pm *PromptManager) RenderUserPromptWithVariables(variables map[string]string) (string, error) {
	// 合并默认变量和指定变量
	mergedVariables := make(map[string]string)
	for k, v := range pm.config.Variables {
		mergedVariables[k] = v
	}
	for k, v := range variables {
		mergedVariables[k] = v
	}

	tmpl, err := template.New("user_prompt").Parse(pm.config.UserPrompt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mergedVariables); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// UpdateVariables 更新提示词变量
func (pm *PromptManager) UpdateVariables(variables map[string]string) {
	for k, v := range variables {
		pm.config.Variables[k] = v
	}
}

// AddVariable 添加单个提示词变量
func (pm *PromptManager) AddVariable(key, value string) {
	pm.config.Variables[key] = value
}

// RemoveVariable 移除单个提示词变量
func (pm *PromptManager) RemoveVariable(key string) {
	delete(pm.config.Variables, key)
}

// GetConfig 获取当前提示词配置
func (pm *PromptManager) GetConfig() PromptConfig {
	return pm.config
}

// SetConfig 设置提示词配置
func (pm *PromptManager) SetConfig(config PromptConfig) {
	pm.config = config
}

// RenderFullPrompt 渲染完整的提示词，包括系统提示词和用户提示词
func (pm *PromptManager) RenderFullPrompt() (string, string, error) {
	systemPrompt := pm.RenderSystemPrompt()
	userPrompt, err := pm.RenderUserPrompt()
	if err != nil {
		return "", "", err
	}

	return systemPrompt, userPrompt, nil
}
