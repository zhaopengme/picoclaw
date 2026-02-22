// MobaiClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 MobaiClaw contributors

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/zhaopengme/mobaiclaw/pkg/bus"
	"github.com/zhaopengme/mobaiclaw/pkg/config"
	"github.com/zhaopengme/mobaiclaw/pkg/constants"
	"github.com/zhaopengme/mobaiclaw/pkg/logger"
	"github.com/zhaopengme/mobaiclaw/pkg/providers"
	"github.com/zhaopengme/mobaiclaw/pkg/routing"
	"github.com/zhaopengme/mobaiclaw/pkg/skills"
	"github.com/zhaopengme/mobaiclaw/pkg/state"
	"github.com/zhaopengme/mobaiclaw/pkg/tools"
	"github.com/zhaopengme/mobaiclaw/pkg/utils"
)

type AgentLoop struct {
	bus         bus.Broker
	cfg         *config.Config
	registry    *AgentRegistry
	state       *state.Manager
	running     atomic.Bool
	summarizing sync.Map
	fallback    *providers.FallbackChain
}

// processOptions configures how a message is processed
type processOptions struct {
	SessionKey      string // Session identifier for history/context
	Channel         string // Target channel for tool execution
	ChatID          string // Target chat ID for tool execution
	UserMessage     string // User message content (may include prefix)
	DefaultResponse string // Response when LLM returns empty
	EnableSummary   bool   // Whether to trigger summarization
	SendResponse    bool   // Whether to send response via bus
	NoHistory       bool   // If true, don't load session history (for heartbeat)
}

func NewAgentLoop(cfg *config.Config, msgBus bus.Broker, provider providers.LLMProvider) *AgentLoop {
	registry := NewAgentRegistry(cfg, provider)

	// Register shared tools to all agents
	registerSharedTools(cfg, msgBus, registry, provider)

	// Set up shared fallback chain
	cooldown := providers.NewCooldownTracker()
	fallbackChain := providers.NewFallbackChain(cooldown)

	// Create state manager using default agent's workspace for channel recording
	defaultAgent := registry.GetDefaultAgent()
	var stateManager *state.Manager
	if defaultAgent != nil {
		stateManager = state.NewManager(defaultAgent.Workspace)
	}

	return &AgentLoop{
		bus:         msgBus,
		cfg:         cfg,
		registry:    registry,
		state:       stateManager,
		summarizing: sync.Map{},
		fallback:    fallbackChain,
	}
}

// registerSharedTools registers tools that are shared across all agents (web, message, spawn).
func registerSharedTools(cfg *config.Config, msgBus bus.Broker, registry *AgentRegistry, provider providers.LLMProvider) {
	for _, agentID := range registry.ListAgentIDs() {
		agent, ok := registry.GetAgent(agentID)
		if !ok {
			continue
		}

		// Web tools
		if searchTool := tools.NewWebSearchTool(tools.WebSearchToolOptions{
			BraveAPIKey:          cfg.Tools.Web.Brave.APIKey,
			BraveMaxResults:      cfg.Tools.Web.Brave.MaxResults,
			BraveEnabled:         cfg.Tools.Web.Brave.Enabled,
			DuckDuckGoMaxResults: cfg.Tools.Web.DuckDuckGo.MaxResults,
			DuckDuckGoEnabled:    cfg.Tools.Web.DuckDuckGo.Enabled,
			PerplexityAPIKey:     cfg.Tools.Web.Perplexity.APIKey,
			PerplexityMaxResults: cfg.Tools.Web.Perplexity.MaxResults,
			PerplexityEnabled:    cfg.Tools.Web.Perplexity.Enabled,
		}); searchTool != nil {
			agent.Tools.Register(searchTool)
		}
		agent.Tools.Register(tools.NewWebFetchTool(50000))

		// Hardware tools (I2C, SPI) - Linux only, returns error on other platforms
		agent.Tools.Register(tools.NewI2CTool())
		agent.Tools.Register(tools.NewSPITool())

		// Message tool
		messageTool := tools.NewMessageTool()
		messageTool.SetSendCallback(func(channel, chatID, content string) error {
			msgBus.PublishOutbound(bus.OutboundMessage{
				Channel: channel,
				ChatID:  chatID,
				Content: content,
			})
			return nil
		})
		agent.Tools.Register(messageTool)

		// Skill discovery and installation tools
		registryMgr := skills.NewRegistryManagerFromConfig(skills.RegistryConfig{
			MaxConcurrentSearches: cfg.Tools.Skills.MaxConcurrentSearches,
			ClawHub:               skills.ClawHubConfig(cfg.Tools.Skills.Registries.ClawHub),
		})
		searchCache := skills.NewSearchCache(cfg.Tools.Skills.SearchCache.MaxSize, time.Duration(cfg.Tools.Skills.SearchCache.TTLSeconds)*time.Second)
		agent.Tools.Register(tools.NewFindSkillsTool(registryMgr, searchCache))
		agent.Tools.Register(tools.NewInstallSkillTool(registryMgr, agent.Workspace))

		// Spawn tool with allowlist checker
		subagentManager := tools.NewSubagentManager(provider, agent.Model, agent.Workspace, msgBus)
		subagentManager.SetLLMOptions(agent.MaxTokens, agent.Temperature)
		spawnTool := tools.NewSpawnTool(subagentManager)
		currentAgentID := agentID
		spawnTool.SetAllowlistChecker(func(targetAgentID string) bool {
			return registry.CanSpawnSubagent(currentAgentID, targetAgentID)
		})
		agent.Tools.Register(spawnTool)

		// Update context builder with the complete tools registry
		agent.ContextBuilder.SetToolsRegistry(agent.Tools)
	}
}

func (al *AgentLoop) Run(ctx context.Context) error {
	al.running.Store(true)

	for al.running.Load() {
		select {
		case <-ctx.Done():
			return nil
		default:
			msg, ok := al.bus.ConsumeInbound(ctx)
			if !ok {
				continue
			}

			response, err := al.processMessage(ctx, msg)
			if err != nil {
				response = fmt.Sprintf("Error processing message: %v", err)
			}

			if response != "" {
				// Check if the message tool already sent a response during this round.
				// If so, skip publishing to avoid duplicate messages to the user.
				// Use default agent's tools to check (message tool is shared).
				alreadySent := false
				defaultAgent := al.registry.GetDefaultAgent()
				if defaultAgent != nil {
					if tool, ok := defaultAgent.Tools.Get("message"); ok {
						if mt, ok := tool.(*tools.MessageTool); ok {
							alreadySent = mt.HasSentInRound()
						}
					}
				}

				if !alreadySent {
					al.bus.PublishOutbound(bus.OutboundMessage{
						Channel: msg.Channel,
						ChatID:  msg.ChatID,
						Content: response,
					})
				}
			}
		}
	}

	return nil
}

func (al *AgentLoop) Stop() {
	al.running.Store(false)
}

func (al *AgentLoop) RegisterTool(tool tools.Tool) {
	for _, agentID := range al.registry.ListAgentIDs() {
		if agent, ok := al.registry.GetAgent(agentID); ok {
			agent.Tools.Register(tool)
		}
	}
}

// RecordLastChannel records the last active channel for this workspace.
// This uses the atomic state save mechanism to prevent data loss on crash.
func (al *AgentLoop) RecordLastChannel(channel string) error {
	if al.state == nil {
		return nil
	}
	return al.state.SetLastChannel(channel)
}

// RecordLastChatID records the last active chat ID for this workspace.
// This uses the atomic state save mechanism to prevent data loss on crash.
func (al *AgentLoop) RecordLastChatID(chatID string) error {
	if al.state == nil {
		return nil
	}
	return al.state.SetLastChatID(chatID)
}

func (al *AgentLoop) ProcessDirect(ctx context.Context, content, sessionKey string) (string, error) {
	return al.ProcessDirectWithChannel(ctx, content, sessionKey, "cli", "direct")
}

func (al *AgentLoop) ProcessDirectWithChannel(ctx context.Context, content, sessionKey, channel, chatID string) (string, error) {
	msg := bus.InboundMessage{
		Channel:    channel,
		SenderID:   "cron",
		ChatID:     chatID,
		Content:    content,
		SessionKey: sessionKey,
	}

	return al.processMessage(ctx, msg)
}

// ProcessHeartbeat processes a heartbeat request without session history.
// Each heartbeat is independent and doesn't accumulate context.
func (al *AgentLoop) ProcessHeartbeat(ctx context.Context, content, channel, chatID string) (string, error) {
	agent := al.registry.GetDefaultAgent()
	return al.runAgentLoop(ctx, agent, processOptions{
		SessionKey:      "heartbeat",
		Channel:         channel,
		ChatID:          chatID,
		UserMessage:     content,
		DefaultResponse: "I've completed processing but have no response to give.",
		EnableSummary:   false,
		SendResponse:    false,
		NoHistory:       true, // Don't load session history for heartbeat
	})
}

func (al *AgentLoop) processMessage(ctx context.Context, msg bus.InboundMessage) (string, error) {
	// Add message preview to log (show full content for error messages)
	var logContent string
	if strings.Contains(msg.Content, "Error:") || strings.Contains(msg.Content, "error") {
		logContent = msg.Content // Full content for errors
	} else {
		logContent = utils.Truncate(msg.Content, 80)
	}
	logger.InfoCF("agent", fmt.Sprintf("Processing message from %s:%s: %s", msg.Channel, msg.SenderID, logContent),
		map[string]interface{}{
			"channel":     msg.Channel,
			"chat_id":     msg.ChatID,
			"sender_id":   msg.SenderID,
			"session_key": msg.SessionKey,
		})

	// Route system messages to processSystemMessage
	if msg.Channel == "system" {
		return al.processSystemMessage(ctx, msg)
	}

	// Route to determine agent and session key
	route := al.registry.ResolveRoute(routing.RouteInput{
		Channel:    msg.Channel,
		AccountID:  msg.Metadata["account_id"],
		Peer:       extractPeer(msg),
		ParentPeer: extractParentPeer(msg),
		GuildID:    msg.Metadata["guild_id"],
		TeamID:     msg.Metadata["team_id"],
	})

	agent, ok := al.registry.GetAgent(route.AgentID)
	if !ok {
		agent = al.registry.GetDefaultAgent()
	}

	// Use routed session key, but honor pre-set agent-scoped keys (for ProcessDirect/cron)
	sessionKey := route.SessionKey
	if msg.SessionKey != "" && strings.HasPrefix(msg.SessionKey, "agent:") {
		sessionKey = msg.SessionKey
		// Extract agent_id from sessionKey and use that agent
		if parsed := routing.ParseAgentSessionKey(sessionKey); parsed != nil {
			if parsedAgent, ok := al.registry.GetAgent(parsed.AgentID); ok {
				agent = parsedAgent
			}
		}
	}

	logger.InfoCF("agent", "Routed message",
		map[string]interface{}{
			"agent_id":    agent.ID,
			"session_key": sessionKey,
			"matched_by":  route.MatchedBy,
		})

	// Intercept built-in commands
	if strings.ToLower(strings.TrimSpace(msg.Content)) == "/clear" {
		// Clear history and summary for the current session
		agent.Sessions.SetHistory(sessionKey, []providers.Message{})
		agent.Sessions.SetSummary(sessionKey, "")
		agent.Sessions.Save(sessionKey)

		// Return a confirmation message directly to the user
		clearMsg := "🧹 当前会话已清空，我们可以重新开始了。"

		// Optional: Log the action
		logger.InfoCF("agent", "User cleared session history",
			map[string]interface{}{
				"agent_id":    agent.ID,
				"session_key": sessionKey,
				"channel":     msg.Channel,
			})

		// return directly without running LLM loop
		return clearMsg, nil
	}

	return al.runAgentLoop(ctx, agent, processOptions{
		SessionKey:      sessionKey,
		Channel:         msg.Channel,
		ChatID:          msg.ChatID,
		UserMessage:     msg.Content,
		DefaultResponse: "I've completed processing but have no response to give.",
		EnableSummary:   true,
		SendResponse:    false,
	})
}

func (al *AgentLoop) processSystemMessage(ctx context.Context, msg bus.InboundMessage) (string, error) {
	if msg.Channel != "system" {
		return "", fmt.Errorf("processSystemMessage called with non-system message channel: %s", msg.Channel)
	}

	logger.InfoCF("agent", "Processing system message",
		map[string]interface{}{
			"sender_id": msg.SenderID,
			"chat_id":   msg.ChatID,
		})

	// Parse origin channel from chat_id (format: "channel:chat_id")
	var originChannel, originChatID string
	if idx := strings.Index(msg.ChatID, ":"); idx > 0 {
		originChannel = msg.ChatID[:idx]
		originChatID = msg.ChatID[idx+1:]
	} else {
		originChannel = "cli"
		originChatID = msg.ChatID
	}

	// Extract subagent result from message content
	// Format: "Task 'label' completed.\n\nResult:\n<actual content>"
	content := msg.Content
	if idx := strings.Index(content, "Result:\n"); idx >= 0 {
		content = content[idx+8:] // Extract just the result part
	}

	// Skip internal channels - only log, don't send to user
	if constants.IsInternalChannel(originChannel) {
		logger.InfoCF("agent", "Subagent completed (internal channel)",
			map[string]interface{}{
				"sender_id":   msg.SenderID,
				"content_len": len(content),
				"channel":     originChannel,
			})
		return "", nil
	}

	// Use default agent for system messages
	agent := al.registry.GetDefaultAgent()

	// Use the origin session for context
	sessionKey := routing.BuildAgentMainSessionKey(agent.ID)

	return al.runAgentLoop(ctx, agent, processOptions{
		SessionKey:      sessionKey,
		Channel:         originChannel,
		ChatID:          originChatID,
		UserMessage:     fmt.Sprintf("[System: %s] %s", msg.SenderID, msg.Content),
		DefaultResponse: "Background task completed.",
		EnableSummary:   false,
		SendResponse:    true,
	})
}

// runAgentLoop is the core message processing logic.
func (al *AgentLoop) runAgentLoop(ctx context.Context, agent *AgentInstance, opts processOptions) (string, error) {
	// 0. Record last channel for heartbeat notifications (skip internal channels)
	if opts.Channel != "" && opts.ChatID != "" {
		// Don't record internal channels (cli, system, subagent)
		if !constants.IsInternalChannel(opts.Channel) {
			channelKey := fmt.Sprintf("%s:%s", opts.Channel, opts.ChatID)
			if err := al.RecordLastChannel(channelKey); err != nil {
				logger.WarnCF("agent", "Failed to record last channel", map[string]interface{}{"error": err.Error()})
			}
		}
	}

	// 1. Update tool contexts
	al.updateToolContexts(agent, opts.Channel, opts.ChatID, opts.SessionKey)

	// 2. Build messages (skip history for heartbeat)
	var history []providers.Message
	var summary string
	if !opts.NoHistory {
		history = agent.Sessions.GetHistory(opts.SessionKey)
		summary = agent.Sessions.GetSummary(opts.SessionKey)
	}
	messages := agent.ContextBuilder.BuildMessages(
		history,
		summary,
		opts.UserMessage,
		nil,
		opts.Channel,
		opts.ChatID,
	)

	// 3. Save user message to session
	agent.Sessions.AddMessage(opts.SessionKey, "user", opts.UserMessage)

	// 4. Run LLM iteration loop
	finalContent, iteration, err := al.runLLMIteration(ctx, agent, messages, opts)
	if err != nil {
		return "", err
	}

	// If last tool had ForUser content and we already sent it, we might not need to send final response
	// This is controlled by the tool's Silent flag and ForUser content

	// 5. Handle empty response
	if finalContent == "" {
		finalContent = opts.DefaultResponse
	}

	// 6. Save final assistant message to session
	agent.Sessions.AddMessage(opts.SessionKey, "assistant", finalContent)
	agent.Sessions.Save(opts.SessionKey)

	// 7. Optional: summarization
	if opts.EnableSummary {
		al.maybeSummarize(agent, opts.SessionKey, opts.Channel, opts.ChatID)
	}

	// 8. Optional: send response via bus
	if opts.SendResponse {
		al.bus.PublishOutbound(bus.OutboundMessage{
			Channel: opts.Channel,
			ChatID:  opts.ChatID,
			Content: finalContent,
		})
	}

	// 9. Log response
	responsePreview := utils.Truncate(finalContent, 120)
	logger.InfoCF("agent", fmt.Sprintf("Response: %s", responsePreview),
		map[string]interface{}{
			"agent_id":     agent.ID,
			"session_key":  opts.SessionKey,
			"iterations":   iteration,
			"final_length": len(finalContent),
		})

	return finalContent, nil
}

// runLLMIteration executes the LLM call loop with tool handling.
func (al *AgentLoop) runLLMIteration(ctx context.Context, agent *AgentInstance, messages []providers.Message, opts processOptions) (string, int, error) {
	iteration := 0
	var finalContent string

	for iteration < agent.MaxIterations {
		iteration++

		logger.DebugCF("agent", "LLM iteration",
			map[string]interface{}{
				"agent_id":  agent.ID,
				"iteration": iteration,
				"max":       agent.MaxIterations,
			})

		// Build tool definitions
		providerToolDefs := agent.Tools.ToProviderDefs()

		// Log LLM request details
		logger.DebugCF("agent", "LLM request",
			map[string]interface{}{
				"agent_id":          agent.ID,
				"iteration":         iteration,
				"model":             agent.Model,
				"messages_count":    len(messages),
				"tools_count":       len(providerToolDefs),
				"max_tokens":        agent.MaxTokens,
				"temperature":       agent.Temperature,
				"system_prompt_len": len(messages[0].Content),
			})

		// Log full messages (detailed)
		logger.DebugCF("agent", "Full LLM request",
			map[string]interface{}{
				"iteration":     iteration,
				"messages_json": formatMessagesForLog(messages),
				"tools_json":    formatToolsForLog(providerToolDefs),
			})

		// Call LLM with fallback chain if candidates are configured.
		var response *providers.LLMResponse
		var err error

		callLLM := func() (*providers.LLMResponse, error) {
			if len(agent.Candidates) > 1 && al.fallback != nil {
				fbResult, fbErr := al.fallback.Execute(ctx, agent.Candidates,
					func(ctx context.Context, provider, model string) (*providers.LLMResponse, error) {
						return agent.Provider.Chat(ctx, messages, providerToolDefs, model, map[string]interface{}{
							"max_tokens":  agent.MaxTokens,
							"temperature": agent.Temperature,
						})
					},
				)
				if fbErr != nil {
					return nil, fbErr
				}
				if fbResult.Provider != "" && len(fbResult.Attempts) > 0 {
					logger.InfoCF("agent", fmt.Sprintf("Fallback: succeeded with %s/%s after %d attempts",
						fbResult.Provider, fbResult.Model, len(fbResult.Attempts)+1),
						map[string]interface{}{"agent_id": agent.ID, "iteration": iteration})
				}
				return fbResult.Response, nil
			}
			return agent.Provider.Chat(ctx, messages, providerToolDefs, agent.Model, map[string]interface{}{
				"max_tokens":  agent.MaxTokens,
				"temperature": agent.Temperature,
			})
		}

		// Retry loop for context/token errors
		maxRetries := 2
		var llmDuration time.Duration
		for retry := 0; retry <= maxRetries; retry++ {
			startTime := time.Now()
			response, err = callLLM()
			llmDuration = time.Since(startTime)
			if err == nil {
				break
			}

			errMsg := strings.ToLower(err.Error())
			isContextError := strings.Contains(errMsg, "token") ||
				strings.Contains(errMsg, "context") ||
				strings.Contains(errMsg, "invalidparameter") ||
				strings.Contains(errMsg, "length")

			if isContextError && retry < maxRetries {
				logger.WarnCF("agent", "Context window error detected, attempting compression", map[string]interface{}{
					"error": err.Error(),
					"retry": retry,
				})

				if retry == 0 && !constants.IsInternalChannel(opts.Channel) {
					al.bus.PublishOutbound(bus.OutboundMessage{
						Channel: opts.Channel,
						ChatID:  opts.ChatID,
						Content: "Context window exceeded. Compressing history and retrying...",
					})
				}

				al.forceCompression(agent, opts.SessionKey)
				newHistory := agent.Sessions.GetHistory(opts.SessionKey)
				newSummary := agent.Sessions.GetSummary(opts.SessionKey)
				messages = agent.ContextBuilder.BuildMessages(
					newHistory, newSummary, "",
					nil, opts.Channel, opts.ChatID,
				)
				continue
			}
			break
		}

		if err != nil {
			logger.ErrorCF("agent", "LLM call failed",
				map[string]interface{}{
					"agent_id":    agent.ID,
					"iteration":   iteration,
					"error":       err.Error(),
					"duration_ms": llmDuration.Milliseconds(),
				})
			return "", iteration, fmt.Errorf("LLM call failed after retries: %w", err)
		}

		// Check if no tool calls - we're done
		if len(response.ToolCalls) == 0 {
			finalContent = response.Content
			logger.InfoCF("agent", "LLM response without tool calls (direct answer)",
				map[string]interface{}{
					"agent_id":      agent.ID,
					"iteration":     iteration,
					"content_chars": len(finalContent),
					"duration_ms":   llmDuration.Milliseconds(),
				})
			break
		}

		normalizedToolCalls := make([]providers.ToolCall, 0, len(response.ToolCalls))
		for _, tc := range response.ToolCalls {
			normalizedToolCalls = append(normalizedToolCalls, providers.NormalizeToolCall(tc))
		}

		// Log tool calls
		toolNames := make([]string, 0, len(normalizedToolCalls))
		for _, tc := range normalizedToolCalls {
			toolNames = append(toolNames, tc.Name)
		}
		logger.InfoCF("agent", "LLM requested tool calls",
			map[string]interface{}{
				"agent_id":    agent.ID,
				"tools":       toolNames,
				"count":       len(normalizedToolCalls),
				"iteration":   iteration,
				"duration_ms": llmDuration.Milliseconds(),
			})

		// Build assistant message with tool calls
		assistantMsg := providers.Message{
			Role:             "assistant",
			Content:          response.Content,
			ReasoningContent: response.ReasoningContent,
		}
		for _, tc := range normalizedToolCalls {
			argumentsJSON, _ := json.Marshal(tc.Arguments)
			// Copy ExtraContent to ensure thought_signature is persisted for Gemini 3
			extraContent := tc.ExtraContent
			thoughtSignature := ""
			if tc.Function != nil {
				thoughtSignature = tc.Function.ThoughtSignature
			}

			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, providers.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Name: tc.Name,
				Function: &providers.FunctionCall{
					Name:             tc.Name,
					Arguments:        string(argumentsJSON),
					ThoughtSignature: thoughtSignature,
				},
				ExtraContent:     extraContent,
				ThoughtSignature: thoughtSignature,
			})
		}
		messages = append(messages, assistantMsg)

		// Save assistant message with tool calls to session
		agent.Sessions.AddFullMessage(opts.SessionKey, assistantMsg)

		// Broadcast status update to channel before running potentially slow tools
		if len(normalizedToolCalls) > 0 && !constants.IsInternalChannel(opts.Channel) {
			var toolNamesDisplay []string
			for _, tc := range normalizedToolCalls {
				toolNamesDisplay = append(toolNamesDisplay, formatToolCallDisplay(tc))
			}

			statusMsg := fmt.Sprintf("⚙️ 正在执行: %s...", strings.Join(toolNamesDisplay, ", "))
			al.bus.PublishOutbound(bus.OutboundMessage{
				Channel:  opts.Channel,
				ChatID:   opts.ChatID,
				Content:  statusMsg,
				Metadata: map[string]string{"status_update": "true"},
			})
		}

		// Execute tool calls
		for _, tc := range normalizedToolCalls {
			argsJSON, _ := json.Marshal(tc.Arguments)
			argsPreview := utils.Truncate(string(argsJSON), 200)
			logger.InfoCF("agent", fmt.Sprintf("Tool call: %s(%s)", tc.Name, argsPreview),
				map[string]interface{}{
					"agent_id":  agent.ID,
					"tool":      tc.Name,
					"iteration": iteration,
				})

			// Create async callback for tools that implement AsyncTool
			// NOTE: Following openclaw's design, async tools do NOT send results directly to users.
			// Instead, they notify the agent via PublishInbound, and the agent decides
			// whether to forward the result to the user (in processSystemMessage).
			asyncCallback := func(callbackCtx context.Context, result *tools.ToolResult) {
				// Log the async completion but don't send directly to user
				// The agent will handle user notification via processSystemMessage
				if !result.Silent && result.ForUser != "" {
					logger.InfoCF("agent", "Async tool completed, agent will handle notification",
						map[string]interface{}{
							"tool":        tc.Name,
							"content_len": len(result.ForUser),
						})
				}
			}

			// Create progress callback with debouncing
			lastUpdate := time.Now()
			var mu sync.Mutex

			progressCallback := func(content string) {
				if constants.IsInternalChannel(opts.Channel) {
					return
				}

				mu.Lock()
				defer mu.Unlock()

				// Debounce: max 1 update every 2 seconds to avoid Telegram rate limits
				if time.Since(lastUpdate) > 2*time.Second {
					statusMsg := fmt.Sprintf("⚙️ 正在执行: %s...\n\n<pre>%s</pre>", formatToolCallDisplay(tc), content)
					al.bus.PublishOutbound(bus.OutboundMessage{
						Channel:  opts.Channel,
						ChatID:   opts.ChatID,
						Content:  statusMsg,
						Metadata: map[string]string{"status_update": "true"},
					})
					lastUpdate = time.Now()
				}
			}

			toolResult := agent.Tools.ExecuteWithContext(ctx, tc.Name, tc.Arguments, opts.Channel, opts.ChatID, opts.SessionKey, asyncCallback, progressCallback)

			// Send ForUser content to user immediately if not Silent
			if !toolResult.Silent && toolResult.ForUser != "" && opts.SendResponse {
				al.bus.PublishOutbound(bus.OutboundMessage{
					Channel: opts.Channel,
					ChatID:  opts.ChatID,
					Content: toolResult.ForUser,
				})
				logger.DebugCF("agent", "Sent tool result to user",
					map[string]interface{}{
						"tool":        tc.Name,
						"content_len": len(toolResult.ForUser),
					})
			}

			// Determine content for LLM based on tool result
			contentForLLM := toolResult.ForLLM
			if contentForLLM == "" && toolResult.Err != nil {
				contentForLLM = toolResult.Err.Error()
			}

			toolResultMsg := providers.Message{
				Role:       "tool",
				Content:    contentForLLM,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolResultMsg)

			// Save tool result message to session
			agent.Sessions.AddFullMessage(opts.SessionKey, toolResultMsg)
		}
	}

	return finalContent, iteration, nil
}

// updateToolContexts updates the context for tools that need channel/chatID/sessionKey info.
func (al *AgentLoop) updateToolContexts(agent *AgentInstance, channel, chatID, sessionKey string) {
	// Use ContextualTool interface instead of type assertions
	if tool, ok := agent.Tools.Get("message"); ok {
		if mt, ok := tool.(tools.ContextualTool); ok {
			mt.SetContext(channel, chatID, sessionKey)
		}
	}
	if tool, ok := agent.Tools.Get("spawn"); ok {
		if st, ok := tool.(tools.ContextualTool); ok {
			st.SetContext(channel, chatID, sessionKey)
		}
	}
	if tool, ok := agent.Tools.Get("subagent"); ok {
		if st, ok := tool.(tools.ContextualTool); ok {
			st.SetContext(channel, chatID, sessionKey)
		}
	}
	if tool, ok := agent.Tools.Get("cron"); ok {
		if ct, ok := tool.(tools.ContextualTool); ok {
			ct.SetContext(channel, chatID, sessionKey)
		}
	}
}

// maybeSummarize triggers summarization if the session history exceeds thresholds.
func (al *AgentLoop) maybeSummarize(agent *AgentInstance, sessionKey, channel, chatID string) {
	newHistory := agent.Sessions.GetHistory(sessionKey)
	tokenEstimate := al.estimateTokens(newHistory)
	threshold := agent.ContextWindow * 75 / 100

	if tokenEstimate > threshold {
		summarizeKey := agent.ID + ":" + sessionKey
		if _, loading := al.summarizing.LoadOrStore(summarizeKey, true); !loading {
			go func() {
				defer al.summarizing.Delete(summarizeKey)
				if !constants.IsInternalChannel(channel) {
					al.bus.PublishOutbound(bus.OutboundMessage{
						Channel: channel,
						ChatID:  chatID,
						Content: "Memory threshold reached. Optimizing conversation history...",
					})
				}
				al.summarizeSession(agent, sessionKey)
			}()
		}
	}
}

// forceCompression aggressively reduces context when the limit is hit.
// It drops the oldest 50% of messages (keeping system prompt and last user message).
func (al *AgentLoop) forceCompression(agent *AgentInstance, sessionKey string) {
	history := agent.Sessions.GetHistory(sessionKey)
	if len(history) <= 4 {
		return
	}

	// Keep system prompt (usually [0]) and the very last message (user's trigger)
	// We want to drop the oldest half of the *conversation*
	// Assuming [0] is system, [1:] is conversation
	conversation := history[1 : len(history)-1]
	if len(conversation) == 0 {
		return
	}

	// Helper to find the mid-point of the conversation
	mid := len(conversation) / 2

	// New history structure:
	// 1. System Prompt (with compression note appended)
	// 2. Second half of conversation
	// 3. Last message

	droppedCount := mid
	keptConversation := conversation[mid:]

	newHistory := make([]providers.Message, 0)

	// Append compression note to the original system prompt instead of adding a new system message
	// This avoids having two consecutive system messages which some APIs (like Zhipu) reject
	compressionNote := fmt.Sprintf("\n\n[System Note: Emergency compression dropped %d oldest messages due to context limit]", droppedCount)
	enhancedSystemPrompt := history[0]
	enhancedSystemPrompt.Content = enhancedSystemPrompt.Content + compressionNote
	newHistory = append(newHistory, enhancedSystemPrompt)

	newHistory = append(newHistory, keptConversation...)
	newHistory = append(newHistory, history[len(history)-1]) // Last message

	// Update session
	agent.Sessions.SetHistory(sessionKey, newHistory)
	agent.Sessions.Save(sessionKey)

	logger.WarnCF("agent", "Forced compression executed", map[string]interface{}{
		"session_key":  sessionKey,
		"dropped_msgs": droppedCount,
		"new_count":    len(newHistory),
	})
}

// GetStartupInfo returns information about loaded tools and skills for logging.
func (al *AgentLoop) GetStartupInfo() map[string]interface{} {
	info := make(map[string]interface{})

	agent := al.registry.GetDefaultAgent()
	if agent == nil {
		return info
	}

	// Tools info
	toolsList := agent.Tools.List()
	info["tools"] = map[string]interface{}{
		"count": len(toolsList),
		"names": toolsList,
	}

	// Skills info
	info["skills"] = agent.ContextBuilder.GetSkillsInfo()

	// Agents info
	info["agents"] = map[string]interface{}{
		"count": len(al.registry.ListAgentIDs()),
		"ids":   al.registry.ListAgentIDs(),
	}

	return info
}

// formatMessagesForLog formats messages for logging
func formatMessagesForLog(messages []providers.Message) string {
	if len(messages) == 0 {
		return "[]"
	}

	var result string
	result += "[\n"
	for i, msg := range messages {
		result += fmt.Sprintf("  [%d] Role: %s\n", i, msg.Role)
		if len(msg.ToolCalls) > 0 {
			result += "  ToolCalls:\n"
			for _, tc := range msg.ToolCalls {
				result += fmt.Sprintf("    - ID: %s, Type: %s, Name: %s\n", tc.ID, tc.Type, tc.Name)
				if tc.Function != nil {
					result += fmt.Sprintf("      Arguments: %s\n", utils.Truncate(tc.Function.Arguments, 200))
				}
			}
		}
		if msg.Content != "" {
			content := utils.Truncate(msg.Content, 200)
			result += fmt.Sprintf("  Content: %s\n", content)
		}
		if msg.ToolCallID != "" {
			result += fmt.Sprintf("  ToolCallID: %s\n", msg.ToolCallID)
		}
		result += "\n"
	}
	result += "]"
	return result
}

// formatToolsForLog formats tool definitions for logging
func formatToolsForLog(tools []providers.ToolDefinition) string {
	if len(tools) == 0 {
		return "[]"
	}

	var result string
	result += "[\n"
	for i, tool := range tools {
		result += fmt.Sprintf("  [%d] Type: %s, Name: %s\n", i, tool.Type, tool.Function.Name)
		result += fmt.Sprintf("      Description: %s\n", tool.Function.Description)
		if len(tool.Function.Parameters) > 0 {
			result += fmt.Sprintf("      Parameters: %s\n", utils.Truncate(fmt.Sprintf("%v", tool.Function.Parameters), 200))
		}
	}
	result += "]"
	return result
}

// summarizeSession summarizes the conversation history for a session.
func (al *AgentLoop) summarizeSession(agent *AgentInstance, sessionKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	history := agent.Sessions.GetHistory(sessionKey)
	summary := agent.Sessions.GetSummary(sessionKey)

	// Keep last 4 messages for continuity
	if len(history) <= 4 {
		return
	}

	toSummarize := history[:len(history)-4]

	// Oversized Message Guard
	maxMessageTokens := agent.ContextWindow / 2
	validMessages := make([]providers.Message, 0)
	omitted := false

	for _, m := range toSummarize {
		switch m.Role {
		case "user", "assistant", "tool":
			// include all conversation roles
		default:
			continue
		}
		msgTokens := al.estimateTokens([]providers.Message{m})
		if msgTokens > maxMessageTokens {
			omitted = true
			continue
		}
		validMessages = append(validMessages, m)
	}

	if len(validMessages) == 0 {
		return
	}

	// Multi-Part Summarization
	var finalSummary string
	if len(validMessages) > 10 {
		mid := len(validMessages) / 2
		part1 := validMessages[:mid]
		part2 := validMessages[mid:]

		var s1, s2 string
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			s1, _ = al.summarizeBatch(ctx, agent, part1, summary) // carry existing context into first batch
		}()

		go func() {
			defer wg.Done()
			s2, _ = al.summarizeBatch(ctx, agent, part2, "")
		}()

		wg.Wait()

		mergePrompt := fmt.Sprintf(
			"Merge these two conversation summaries into one JSON object with fields: overview, scheduled_tasks, preferences, pending_actions, key_facts.\n"+
				"Return ONLY the JSON object, no markdown fences, no other text.\n\nSummary 1: %s\n\nSummary 2: %s",
			s1, s2,
		)
		summaryModel := agent.SummaryModel
		if summaryModel == "" {
			summaryModel = agent.Model
		}
		resp, err := agent.Provider.Chat(ctx, []providers.Message{{Role: "user", Content: mergePrompt}}, nil, summaryModel, map[string]interface{}{
			"max_tokens":  1024,
			"temperature": 0.3,
		})
		if err == nil {
			finalSummary = resp.Content
		} else {
			// merge failed: use s1 which already carries the existing context
			logger.WarnCF("agent", "Summary merge failed, falling back to part1 summary", map[string]interface{}{"error": err.Error()})
			finalSummary = s1
		}
	} else {
		finalSummary, _ = al.summarizeBatch(ctx, agent, validMessages, summary)
	}

	if omitted && finalSummary != "" {
		if cs, ok := parseSummary(finalSummary); ok {
			if cs.Overview != "" {
				cs.Overview += " [Note: some oversized messages were omitted.]"
			} else {
				cs.Overview = "[Note: some oversized messages were omitted.]"
			}
			if b, err := json.Marshal(cs); err == nil {
				finalSummary = string(b)
			}
		}
	}

	if finalSummary != "" {
		if _, ok := parseSummary(finalSummary); ok {
			agent.Sessions.SetSummary(sessionKey, finalSummary)
		} else {
			logger.WarnCF("agent", "Summary is not valid JSON, truncating history without saving summary", map[string]interface{}{
				"session_key": sessionKey,
				"preview":     finalSummary[:min(len(finalSummary), 100)],
			})
		}
		// Always truncate history to prevent repeated summarization cycles
		agent.Sessions.TruncateHistory(sessionKey, 4)
		agent.Sessions.Save(sessionKey)
	}
}

// summarizeBatch summarizes a batch of messages.
func (al *AgentLoop) summarizeBatch(ctx context.Context, agent *AgentInstance, batch []providers.Message, existingSummary string) (string, error) {
	prompt := "Summarize this conversation segment as a JSON object with these fields:\n" +
		"- \"overview\": one paragraph of key context and outcomes\n" +
		"- \"scheduled_tasks\": list of tasks/reminders created with IDs if known (empty array if none)\n" +
		"- \"preferences\": user preferences and settings stated (empty array if none)\n" +
		"- \"pending_actions\": items that need follow-up (empty array if none)\n" +
		"- \"key_facts\": important facts established (empty array if none)\n" +
		"Return ONLY the JSON object, no markdown fences, no other text.\n"
	if cs, ok := parseSummary(existingSummary); ok {
		prompt += "Existing context: " + renderSummaryText(cs) + "\n"
	}
	prompt += "\nCONVERSATION:\n"
	for _, m := range batch {
		if text := messageToSummaryText(m); text != "" {
			prompt += text + "\n"
		}
	}

	summaryModel := agent.SummaryModel
	if summaryModel == "" {
		summaryModel = agent.Model
	}
	response, err := agent.Provider.Chat(ctx, []providers.Message{{Role: "user", Content: prompt}}, nil, summaryModel, map[string]interface{}{
		"max_tokens":  1024,
		"temperature": 0.3,
	})
	if err != nil {
		return "", err
	}
	return response.Content, nil
}

// messageToSummaryText converts a message to a readable text line for summarization.
// Tool calls and results are formatted to preserve intent without raw JSON noise.
func messageToSummaryText(m providers.Message) string {
	switch m.Role {
	case "user":
		if m.Content == "" {
			return ""
		}
		return "user: " + m.Content

	case "assistant":
		var parts []string
		if m.Content != "" {
			parts = append(parts, m.Content)
		}
		for _, tc := range m.ToolCalls {
			name := tc.Name
			if name == "" && tc.Function != nil {
				name = tc.Function.Name
			}
			args := ""
			if tc.Function != nil && tc.Function.Arguments != "" {
				args = tc.Function.Arguments
			} else if len(tc.Arguments) > 0 {
				if b, err := json.Marshal(tc.Arguments); err == nil {
					args = string(b)
				}
			}
			if argRunes := []rune(args); len(argRunes) > 200 {
				args = string(argRunes[:200]) + "..."
			}
			parts = append(parts, fmt.Sprintf("[Tool Call: %s(%s)]", name, args))
		}
		if len(parts) == 0 {
			return ""
		}
		return "assistant: " + strings.Join(parts, " ")

	case "tool":
		runes := []rune(m.Content)
		if len(runes) > 300 {
			runes = runes[:300]
			return fmt.Sprintf("[Tool Result]: %s...", string(runes))
		}
		return fmt.Sprintf("[Tool Result]: %s", m.Content)

	default:
		return ""
	}
}

// ConversationSummary is the structured format for conversation summaries.
type ConversationSummary struct {
	Overview       string   `json:"overview"`
	ScheduledTasks []string `json:"scheduled_tasks,omitempty"`
	Preferences    []string `json:"preferences,omitempty"`
	PendingActions []string `json:"pending_actions,omitempty"`
	KeyFacts       []string `json:"key_facts,omitempty"`
}

// parseSummary tries to parse a summary string as structured JSON.
// Returns the parsed summary and true if successful, zero value and false otherwise.
func parseSummary(s string) (ConversationSummary, bool) {
	s = strings.TrimSpace(s)
	// Strip markdown code fences if LLM wrapped in ```json ... ``` or ``` ... ```
	if strings.HasPrefix(s, "```") && strings.HasSuffix(s, "```") && len(s) > 6 {
		inner := s[3 : len(s)-3]
		inner = strings.TrimSpace(inner)
		// Remove optional language hint on the first line (e.g., "json")
		if nl := strings.Index(inner, "\n"); nl >= 0 {
			firstLine := strings.TrimSpace(inner[:nl])
			if !strings.Contains(firstLine, "{") {
				inner = strings.TrimSpace(inner[nl+1:])
			}
		}
		s = inner
	}
	var cs ConversationSummary
	if err := json.Unmarshal([]byte(s), &cs); err != nil {
		return ConversationSummary{}, false
	}
	return cs, true
}

// renderSummaryText renders a ConversationSummary as plain text for LLM consumption (e.g., as existing context).
func renderSummaryText(cs ConversationSummary) string {
	var sb strings.Builder
	if cs.Overview != "" {
		sb.WriteString(cs.Overview)
	}
	if len(cs.ScheduledTasks) > 0 {
		sb.WriteString("\nScheduled tasks: " + strings.Join(cs.ScheduledTasks, "; "))
	}
	if len(cs.Preferences) > 0 {
		sb.WriteString("\nPreferences: " + strings.Join(cs.Preferences, "; "))
	}
	if len(cs.PendingActions) > 0 {
		sb.WriteString("\nPending actions: " + strings.Join(cs.PendingActions, "; "))
	}
	if len(cs.KeyFacts) > 0 {
		sb.WriteString("\nKey facts: " + strings.Join(cs.KeyFacts, "; "))
	}
	return sb.String()
}

// renderSummaryMarkdown renders a ConversationSummary as formatted markdown for the system prompt.
func renderSummaryMarkdown(cs ConversationSummary) string {
	var sb strings.Builder
	if cs.Overview != "" {
		sb.WriteString(cs.Overview + "\n")
	}
	if len(cs.ScheduledTasks) > 0 {
		sb.WriteString("\n**Scheduled Tasks:**\n")
		for _, t := range cs.ScheduledTasks {
			sb.WriteString("- " + t + "\n")
		}
	}
	if len(cs.Preferences) > 0 {
		sb.WriteString("\n**User Preferences:**\n")
		for _, p := range cs.Preferences {
			sb.WriteString("- " + p + "\n")
		}
	}
	if len(cs.PendingActions) > 0 {
		sb.WriteString("\n**Pending Actions:**\n")
		for _, a := range cs.PendingActions {
			sb.WriteString("- " + a + "\n")
		}
	}
	if len(cs.KeyFacts) > 0 {
		sb.WriteString("\n**Key Facts:**\n")
		for _, f := range cs.KeyFacts {
			sb.WriteString("- " + f + "\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

// estimateTokens estimates the number of tokens in a message list.
// Uses a safe heuristic of 2.5 characters per token to account for CJK and other
// overheads better than the previous 3 chars/token.
// Also accounts for tool call payloads (function name + arguments) which are
// stored in ToolCalls rather than Content for assistant messages.
func (al *AgentLoop) estimateTokens(messages []providers.Message) int {
	totalChars := 0
	for _, m := range messages {
		totalChars += utf8.RuneCountInString(m.Content)
		for _, tc := range m.ToolCalls {
			// Count function name
			if tc.Function != nil {
				totalChars += utf8.RuneCountInString(tc.Function.Name)
				totalChars += utf8.RuneCountInString(tc.Function.Arguments)
			} else {
				totalChars += utf8.RuneCountInString(tc.Name)
				if len(tc.Arguments) > 0 {
					if b, err := json.Marshal(tc.Arguments); err == nil {
						totalChars += len(b)
					}
				}
			}
		}
	}
	// 2.5 chars per token = totalChars * 2 / 5
	return totalChars * 2 / 5
}

// extractPeer extracts the routing peer from inbound message metadata.
func extractPeer(msg bus.InboundMessage) *routing.RoutePeer {
	peerKind := msg.Metadata["peer_kind"]
	if peerKind == "" {
		return nil
	}
	peerID := msg.Metadata["peer_id"]
	if peerID == "" {
		if peerKind == "direct" {
			peerID = msg.SenderID
		} else {
			peerID = msg.ChatID
		}
	}
	return &routing.RoutePeer{Kind: peerKind, ID: peerID}
}

// extractParentPeer extracts the parent peer (reply-to) from inbound message metadata.
func extractParentPeer(msg bus.InboundMessage) *routing.RoutePeer {
	parentKind := msg.Metadata["parent_peer_kind"]
	parentID := msg.Metadata["parent_peer_id"]
	if parentKind == "" || parentID == "" {
		return nil
	}
	return &routing.RoutePeer{Kind: parentKind, ID: parentID}
}

// GetRegistry returns the agent registry for external usage.
func (al *AgentLoop) GetRegistry() *AgentRegistry {
	return al.registry
}

// formatToolCallDisplay formats a tool call for UI display, extracting key arguments
func formatToolCallDisplay(tc providers.ToolCall) string {
	if tc.Name == "exec" {
		if cmd, ok := tc.Arguments["command"].(string); ok {
			// Clean up newlines for display
			cmd = strings.ReplaceAll(cmd, "\n", " ")
			if len(cmd) > 40 {
				cmd = cmd[:37] + "..."
			}
			return fmt.Sprintf("%s(`%s`)", tc.Name, cmd)
		}
	}

	// Fallback for other tools: show a brief JSON snippet
	argsJSON, err := json.Marshal(tc.Arguments)
	if err == nil && len(argsJSON) > 2 && len(argsJSON) < 60 {
		return fmt.Sprintf("%s(%s)", tc.Name, string(argsJSON))
	} else if err == nil && len(argsJSON) >= 60 {
		return fmt.Sprintf("%s(%s...)", tc.Name, string(argsJSON[:50]))
	}

	return tc.Name
}
