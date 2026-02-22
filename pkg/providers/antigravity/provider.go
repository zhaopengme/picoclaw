// MobaiClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 MobaiClaw contributors

package antigravity

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/zhaopengme/mobaiclaw/pkg/auth"
	"github.com/zhaopengme/mobaiclaw/pkg/logger"
	"github.com/zhaopengme/mobaiclaw/pkg/providers/protocoltypes"
)

const (
	baseURL      = "https://cloudcode-pa.googleapis.com"
	defaultModel = "gemini-3-flash"
	userAgent    = "antigravity"
	xGoogClient  = "google-cloud-sdk vscode_cloudshelleditor/0.1"
	version      = "1.15.8"
)

type Provider struct {
	tokenSource func() (string, string, error)
	httpClient  *http.Client
}

func NewProvider() *Provider {
	return &Provider{
		tokenSource: createTokenSource(),
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *Provider) Chat(ctx context.Context, messages []protocoltypes.Message, tools []protocoltypes.ToolDefinition, model string, options map[string]interface{}) (*protocoltypes.LLMResponse, error) {
	accessToken, projectID, err := p.tokenSource()
	if err != nil {
		return nil, fmt.Errorf("antigravity auth: %w", err)
	}

	if model == "" || model == "antigravity" || model == "google-antigravity" {
		model = defaultModel
	}
	model = strings.TrimPrefix(model, "google-antigravity/")
	model = strings.TrimPrefix(model, "antigravity/")

	logger.DebugCF("provider.antigravity", "Starting chat", map[string]interface{}{
		"model":     model,
		"project":   projectID,
		"requestId": fmt.Sprintf("agent-%d-%s", time.Now().UnixMilli(), randomString(9)),
	})

	innerRequest := p.buildRequest(messages, tools, model, options)

	envelope := map[string]interface{}{
		"project":     projectID,
		"model":       model,
		"request":     innerRequest,
		"requestType": "agent",
		"userAgent":   userAgent,
		"requestId":   fmt.Sprintf("agent-%d-%s", time.Now().UnixMilli(), randomString(9)),
	}

	bodyBytes, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/v1internal:streamGenerateContent?alt=sse", baseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	clientMetadata, _ := json.Marshal(map[string]string{
		"ideType":    "IDE_UNSPECIFIED",
		"platform":   "PLATFORM_UNSPECIFIED",
		"pluginType": "GEMINI",
	})
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", fmt.Sprintf("antigravity/%s linux/amd64", version))
	req.Header.Set("X-Goog-Api-Client", xGoogClient)
	req.Header.Set("Client-Metadata", string(clientMetadata))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("antigravity API call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.ErrorCF("provider.antigravity", "API call failed", map[string]interface{}{
			"status_code": resp.StatusCode,
			"response":    string(respBody),
			"model":       model,
		})

		return nil, p.parseError(resp.StatusCode, respBody)
	}

	llmResp, err := p.parseSSEResponse(string(respBody))
	if err != nil {
		return nil, err
	}

	if llmResp.Content == "" && len(llmResp.ToolCalls) == 0 {
		return nil, fmt.Errorf("antigravity: model returned an empty response (this model might be invalid or restricted)")
	}

	return llmResp, nil
}

func (p *Provider) GetDefaultModel() string {
	return defaultModel
}

type request struct {
	Contents     []content     `json:"contents"`
	Tools        []tool        `json:"tools,omitempty"`
	SystemPrompt *systemPrompt `json:"systemInstruction,omitempty"`
	Config       *genConfig    `json:"generationConfig,omitempty"`
}

type content struct {
	Role  string `json:"role"`
	Parts []part `json:"parts"`
}

type part struct {
	Text                  string                `json:"text,omitempty"`
	ThoughtSignature      string                `json:"thoughtSignature,omitempty"`
	ThoughtSignatureSnake string                `json:"thought_signature,omitempty"`
	FunctionCall          *functionCall         `json:"functionCall,omitempty"`
	FunctionResponse      *functionResponse     `json:"functionResponse,omitempty"`
}

type functionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type functionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

type tool struct {
	FunctionDeclarations []functionDecl `json:"functionDeclarations"`
}

type functionDecl struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type systemPrompt struct {
	Parts []part `json:"parts"`
}

type genConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

func (p *Provider) buildRequest(messages []protocoltypes.Message, tools []protocoltypes.ToolDefinition, model string, options map[string]interface{}) request {
	req := request{}
	toolCallNames := make(map[string]string)

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			req.SystemPrompt = &systemPrompt{
				Parts: []part{{Text: msg.Content}},
			}
		case "user":
			if msg.ToolCallID != "" {
				toolName := resolveToolResponseName(msg.ToolCallID, toolCallNames)
				req.Contents = append(req.Contents, content{
					Role: "user",
					Parts: []part{{
						FunctionResponse: &functionResponse{
							Name: toolName,
							Response: map[string]interface{}{
								"result": msg.Content,
							},
						},
					}},
				})
			} else {
				req.Contents = append(req.Contents, content{
					Role:  "user",
					Parts: []part{{Text: msg.Content}},
				})
			}
		case "assistant":
			c := content{
				Role: "model",
			}
			if msg.Content != "" {
				c.Parts = append(c.Parts, part{Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				toolName, toolArgs, thoughtSignature := normalizeToolCall(tc)
				if toolName == "" {
					logger.WarnCF("provider.antigravity", "Skipping tool call with empty name in history", map[string]interface{}{
						"tool_call_id": tc.ID,
					})
					continue
				}
				if tc.ID != "" {
					toolCallNames[tc.ID] = toolName
				}
				c.Parts = append(c.Parts, part{
					ThoughtSignature:      thoughtSignature,
					ThoughtSignatureSnake: thoughtSignature,
					FunctionCall: &functionCall{
						Name: toolName,
						Args: toolArgs,
					},
				})
			}
			if len(c.Parts) > 0 {
				req.Contents = append(req.Contents, c)
			}
		case "tool":
			toolName := resolveToolResponseName(msg.ToolCallID, toolCallNames)
			req.Contents = append(req.Contents, content{
				Role: "user",
				Parts: []part{{
					FunctionResponse: &functionResponse{
						Name: toolName,
						Response: map[string]interface{}{
							"result": msg.Content,
						},
					},
				}},
			})
		}
	}

	if len(tools) > 0 {
		var funcDecls []functionDecl
		for _, t := range tools {
			if t.Type != "function" {
				continue
			}
			params := sanitizeSchema(t.Function.Parameters)
			funcDecls = append(funcDecls, functionDecl{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  params,
			})
		}
		if len(funcDecls) > 0 {
			req.Tools = []tool{{FunctionDeclarations: funcDecls}}
		}
	}

	config := &genConfig{}
	if val, ok := options["max_tokens"]; ok {
		if maxTokens, ok := val.(int); ok && maxTokens > 0 {
			config.MaxOutputTokens = maxTokens
		} else if maxTokens, ok := val.(float64); ok && maxTokens > 0 {
			config.MaxOutputTokens = int(maxTokens)
		}
	}
	if temp, ok := options["temperature"].(float64); ok {
		config.Temperature = temp
	}
	if config.MaxOutputTokens > 0 || config.Temperature > 0 {
		req.Config = config
	}

	return req
}

func normalizeToolCall(tc protocoltypes.ToolCall) (string, map[string]interface{}, string) {
	name := tc.Name
	args := tc.Arguments
	thoughtSignature := ""

	if name == "" && tc.Function != nil {
		name = tc.Function.Name
		thoughtSignature = tc.Function.ThoughtSignature
	} else if tc.Function != nil {
		thoughtSignature = tc.Function.ThoughtSignature
	}

	if args == nil {
		args = map[string]interface{}{}
	}

	if len(args) == 0 && tc.Function != nil && tc.Function.Arguments != "" {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &parsed); err == nil && parsed != nil {
			args = parsed
		}
	}

	return name, args, thoughtSignature
}

func resolveToolResponseName(toolCallID string, toolCallNames map[string]string) string {
	if toolCallID == "" {
		return ""
	}

	if name, ok := toolCallNames[toolCallID]; ok && name != "" {
		return name
	}

	return inferToolNameFromCallID(toolCallID)
}

func inferToolNameFromCallID(toolCallID string) string {
	if !strings.HasPrefix(toolCallID, "call_") {
		return toolCallID
	}

	rest := strings.TrimPrefix(toolCallID, "call_")
	if idx := strings.LastIndex(rest, "_"); idx > 0 {
		candidate := rest[:idx]
		if candidate != "" {
			return candidate
		}
	}

	return toolCallID
}

type jsonResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text                  string        `json:"text,omitempty"`
				ThoughtSignature      string        `json:"thoughtSignature,omitempty"`
				ThoughtSignatureSnake string        `json:"thought_signature,omitempty"`
				FunctionCall          *functionCall `json:"functionCall,omitempty"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (p *Provider) parseSSEResponse(body string) (*protocoltypes.LLMResponse, error) {
	var contentParts []string
	var toolCalls []protocoltypes.ToolCall
	var usage *protocoltypes.UsageInfo
	var finishReason string

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var sseChunk struct {
			Response jsonResp `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &sseChunk); err != nil {
			continue
		}
		resp := sseChunk.Response

		for _, candidate := range resp.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					contentParts = append(contentParts, part.Text)
				}
				if part.FunctionCall != nil {
					argumentsJSON, _ := json.Marshal(part.FunctionCall.Args)
					toolCalls = append(toolCalls, protocoltypes.ToolCall{
						ID:        fmt.Sprintf("call_%s_%d", part.FunctionCall.Name, time.Now().UnixNano()),
						Name:      part.FunctionCall.Name,
						Arguments: part.FunctionCall.Args,
						Function: &protocoltypes.FunctionCall{
							Name:             part.FunctionCall.Name,
							Arguments:        string(argumentsJSON),
							ThoughtSignature: extractThoughtSignature(part.ThoughtSignature, part.ThoughtSignatureSnake),
						},
					})
				}
			}
			if candidate.FinishReason != "" {
				finishReason = candidate.FinishReason
			}
		}

		if resp.UsageMetadata.TotalTokenCount > 0 {
			usage = &protocoltypes.UsageInfo{
				PromptTokens:     resp.UsageMetadata.PromptTokenCount,
				CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
				TotalTokens:      resp.UsageMetadata.TotalTokenCount,
			}
		}
	}

	mappedFinish := "stop"
	if len(toolCalls) > 0 {
		mappedFinish = "tool_calls"
	}
	if finishReason == "MAX_TOKENS" {
		mappedFinish = "length"
	}

	return &protocoltypes.LLMResponse{
		Content:      strings.Join(contentParts, ""),
		ToolCalls:    toolCalls,
		FinishReason: mappedFinish,
		Usage:        usage,
	}, nil
}

func extractThoughtSignature(thoughtSignature string, thoughtSignatureSnake string) string {
	if thoughtSignature != "" {
		return thoughtSignature
	}
	if thoughtSignatureSnake != "" {
		return thoughtSignatureSnake
	}
	return ""
}

var geminiUnsupportedKeywords = map[string]bool{
	"patternProperties":    true,
	"additionalProperties": true,
	"$schema":              true,
	"$id":                  true,
	"$ref":                 true,
	"$defs":                true,
	"definitions":          true,
	"examples":             true,
	"minLength":            true,
	"maxLength":            true,
	"minimum":              true,
	"maximum":              true,
	"multipleOf":           true,
	"pattern":              true,
	"format":               true,
	"minItems":             true,
	"maxItems":             true,
	"uniqueItems":          true,
	"minProperties":        true,
	"maxProperties":        true,
}

func sanitizeSchema(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return nil
	}

	result := make(map[string]interface{})
	for k, v := range schema {
		if geminiUnsupportedKeywords[k] {
			continue
		}
		switch val := v.(type) {
		case map[string]interface{}:
			result[k] = sanitizeSchema(val)
		case []interface{}:
			sanitized := make([]interface{}, len(val))
			for i, item := range val {
				if m, ok := item.(map[string]interface{}); ok {
					sanitized[i] = sanitizeSchema(m)
				} else {
					sanitized[i] = item
				}
			}
			result[k] = sanitized
		default:
			result[k] = v
		}
	}

	if _, hasProps := result["properties"]; hasProps {
		if _, hasType := result["type"]; !hasType {
			result["type"] = "object"
		}
	}

	return result
}

func createTokenSource() func() (string, string, error) {
	return func() (string, string, error) {
		cred, err := auth.GetCredential("google-antigravity")
		if err != nil {
			return "", "", fmt.Errorf("loading auth credentials: %w", err)
		}
		if cred == nil {
			return "", "", fmt.Errorf("no credentials for google-antigravity. Run: mobaiclaw auth login --provider google-antigravity")
		}

		if cred.NeedsRefresh() && cred.RefreshToken != "" {
			oauthCfg := auth.GoogleAntigravityOAuthConfig()
			refreshed, err := auth.RefreshAccessToken(cred, oauthCfg)
			if err != nil {
				return "", "", fmt.Errorf("refreshing token: %w", err)
			}
			refreshed.Email = cred.Email
			if refreshed.ProjectID == "" {
				refreshed.ProjectID = cred.ProjectID
			}
			if err := auth.SetCredential("google-antigravity", refreshed); err != nil {
				return "", "", fmt.Errorf("saving refreshed token: %w", err)
			}
			cred = refreshed
		}

		if cred.IsExpired() {
			return "", "", fmt.Errorf("antigravity credentials expired. Run: mobaiclaw auth login --provider google-antigravity")
		}

		projectID := cred.ProjectID
		if projectID == "" {
			fetchedID, err := FetchProjectID(cred.AccessToken)
			if err != nil {
				logger.WarnCF("provider.antigravity", "Could not fetch project ID, using fallback", map[string]interface{}{
					"error": err.Error(),
				})
				projectID = "rising-fact-p41fc"
			} else {
				projectID = fetchedID
				cred.ProjectID = projectID
				_ = auth.SetCredential("google-antigravity", cred)
			}
		}

		return cred.AccessToken, projectID, nil
	}
}

func FetchProjectID(accessToken string) (string, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"ideType":    "IDE_UNSPECIFIED",
			"platform":   "PLATFORM_UNSPECIFIED",
			"pluginType": "GEMINI",
		},
	})

	req, err := http.NewRequest("POST", baseURL+"/v1internal:loadCodeAssist", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Goog-Api-Client", xGoogClient)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("loadCodeAssist failed: %s", string(body))
	}

	var result struct {
		CloudAICompanionProject string `json:"cloudaicompanionProject"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.CloudAICompanionProject == "" {
		return "", fmt.Errorf("no project ID in loadCodeAssist response")
	}

	return result.CloudAICompanionProject, nil
}

func FetchModels(accessToken, projectID string) ([]ModelInfo, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"project": projectID,
	})

	req, err := http.NewRequest("POST", baseURL+"/v1internal:fetchAvailableModels", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Goog-Api-Client", xGoogClient)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchAvailableModels failed (HTTP %d): %s", resp.StatusCode, truncateString(string(body), 200))
	}

	var result struct {
		Models map[string]struct {
			DisplayName string `json:"displayName"`
			QuotaInfo   struct {
				RemainingFraction interface{} `json:"remainingFraction"`
				ResetTime         string      `json:"resetTime"`
				IsExhausted       bool        `json:"isExhausted"`
			} `json:"quotaInfo"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing models response: %w", err)
	}

	var models []ModelInfo
	for id, info := range result.Models {
		models = append(models, ModelInfo{
			ID:          id,
			DisplayName: info.DisplayName,
			IsExhausted: info.QuotaInfo.IsExhausted,
		})
	}

	hasFlashPreview := false
	hasFlash := false
	for _, m := range models {
		if m.ID == "gemini-3-flash-preview" {
			hasFlashPreview = true
		}
		if m.ID == "gemini-3-flash" {
			hasFlash = true
		}
	}
	if !hasFlashPreview {
		models = append(models, ModelInfo{
			ID:          "gemini-3-flash-preview",
			DisplayName: "Gemini 3 Flash (Preview)",
		})
	}
	if !hasFlash {
		models = append(models, ModelInfo{
			ID:          "gemini-3-flash",
			DisplayName: "Gemini 3 Flash",
		})
	}

	return models, nil
}

type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	IsExhausted bool   `json:"is_exhausted"`
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (p *Provider) parseError(statusCode int, body []byte) error {
	var errResp struct {
		Error struct {
			Code    int                      `json:"code"`
			Message string                   `json:"message"`
			Status  string                   `json:"status"`
			Details []map[string]interface{} `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("antigravity API error (HTTP %d): %s", statusCode, truncateString(string(body), 500))
	}

	msg := errResp.Error.Message
	if statusCode == 429 {
		for _, detail := range errResp.Error.Details {
			if typeVal, ok := detail["@type"].(string); ok && strings.HasSuffix(typeVal, "ErrorInfo") {
				if metadata, ok := detail["metadata"].(map[string]interface{}); ok {
					if delay, ok := metadata["quotaResetDelay"].(string); ok {
						return fmt.Errorf("antigravity rate limit exceeded: %s (reset in %s)", msg, delay)
					}
				}
			}
		}
		return fmt.Errorf("antigravity rate limit exceeded: %s", msg)
	}

	return fmt.Errorf("antigravity API error (%s): %s", errResp.Error.Status, msg)
}
