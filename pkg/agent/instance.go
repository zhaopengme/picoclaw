package agent

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/zhaopengme/mobaiclaw/pkg/bus"
	"github.com/zhaopengme/mobaiclaw/pkg/config"
	"github.com/zhaopengme/mobaiclaw/pkg/providers"
	"github.com/zhaopengme/mobaiclaw/pkg/routing"
	"github.com/zhaopengme/mobaiclaw/pkg/session"
	"github.com/zhaopengme/mobaiclaw/pkg/tools"
)

// AgentInstance represents a fully configured agent with its own workspace,
// session manager, context builder, and tool registry.
type AgentInstance struct {
	ID             string
	Name           string
	Model          string
	Fallbacks      []string
	Workspace      string
	MaxIterations  int
	MaxTokens      int
	Temperature    float64
	ContextWindow  int
	SummaryModel   string
	Provider       providers.LLMProvider
	Sessions       *session.SessionManager
	ContextBuilder *ContextBuilder
	Tools          *tools.ToolRegistry
	Subagents      *config.SubagentsConfig
	SkillsFilter   []string
	Candidates     []providers.FallbackCandidate
}

// NewAgentInstance creates an agent instance from config.
func NewAgentInstance(
	agentCfg *config.AgentConfig,
	defaults *config.AgentDefaults,
	cfg *config.Config,
	provider providers.LLMProvider,
) *AgentInstance {
	workspace := resolveAgentWorkspace(agentCfg, defaults)
	os.MkdirAll(workspace, 0755)

	model := resolveAgentModel(agentCfg, defaults)
	fallbacks := resolveAgentFallbacks(agentCfg, defaults)

	restrict := defaults.RestrictToWorkspace
	toolsRegistry := tools.NewToolRegistry()
	toolsRegistry.Register(tools.NewReadFileTool(workspace, restrict))
	toolsRegistry.Register(tools.NewWriteFileTool(workspace, restrict))
	toolsRegistry.Register(tools.NewListDirTool(workspace, restrict))
	toolsRegistry.Register(tools.NewExecToolWithConfig(workspace, restrict, cfg))
	toolsRegistry.Register(tools.NewEditFileTool(workspace, restrict))
	toolsRegistry.Register(tools.NewAppendFileTool(workspace, restrict))

	sessionsDir := filepath.Join(workspace, "sessions")
	sessionsManager := session.NewSessionManager(sessionsDir)

	memoryStore := NewMemoryStore(workspace)
	contextBuilder := NewContextBuilder(workspace, memoryStore)
	contextBuilder.SetToolsRegistry(toolsRegistry)

	// Register memory tools
	toolsRegistry.Register(tools.NewMemoryStoreTool(memoryStore))
	toolsRegistry.Register(tools.NewMemoryDeleteTool(memoryStore))

	agentID := routing.DefaultAgentID
	agentName := ""
	var subagents *config.SubagentsConfig
	var skillsFilter []string

	if agentCfg != nil {
		agentID = routing.NormalizeAgentID(agentCfg.ID)
		agentName = agentCfg.Name
		subagents = agentCfg.Subagents
		skillsFilter = agentCfg.Skills
	}

	maxIter := defaults.MaxToolIterations
	if maxIter == 0 {
		maxIter = 20
	}

	maxTokens := defaults.MaxTokens
	if maxTokens == 0 {
		maxTokens = 8192
	}

	contextWindow := defaults.ContextWindow
	if contextWindow == 0 {
		contextWindow = 32768
	}

	temperature := 0.7
	if defaults.Temperature != nil {
		temperature = *defaults.Temperature
	}

	// Resolve fallback candidates
	modelCfg := providers.ModelConfig{
		Primary:   model,
		Fallbacks: fallbacks,
	}
	candidates := providers.ResolveCandidates(modelCfg, defaults.Provider)

	// Try to create a specific provider for this agent's model
	agentProvider := provider // Fallback to the default provider
	if cfg != nil && model != "" {
		if modelCfg, err := cfg.GetModelConfig(model); err == nil {
			if modelCfg.Workspace == "" {
				modelCfg.Workspace = workspace
			}
			if p, _, err := providers.CreateProviderFromConfig(modelCfg); err == nil {
				agentProvider = p
			} else {
				// We don't import logger here to avoid import cycle, or we could.
				// Just fall back silently to the default provider for now.
			}
		}
	}

	return &AgentInstance{
		ID:             agentID,
		Name:           agentName,
		Model:          model,
		Fallbacks:      fallbacks,
		Workspace:      workspace,
		MaxIterations:  maxIter,
		MaxTokens:      maxTokens,
		Temperature:    temperature,
		ContextWindow:  contextWindow,
		SummaryModel:   defaults.SummaryModel,
		Provider:       agentProvider,
		Sessions:       sessionsManager,
		ContextBuilder: contextBuilder,
		Tools:          toolsRegistry,
		Subagents:      subagents,
		SkillsFilter:   skillsFilter,
		Candidates:     candidates,
	}
}

// SetupAgentTools holds the function reference for tool registration.
// It is set by the AgentLoop during initialization.
// This function is defined in loop.go to avoid import cycles.
type AgentSetupFunc func(agentID string, instance *AgentInstance, cfg *config.Config, msgBus bus.Broker, registry *AgentRegistry, provider providers.LLMProvider)

var SetupAgentTools AgentSetupFunc = nil

// extraTools holds tools registered externally (e.g. CronTool from cmd_gateway)
// that need to survive registry reload. setupAgentTools registers these automatically.
// Must only be accessed while holding extraToolsMu.
var (
	extraTools   []tools.Tool
	extraToolsMu sync.Mutex
)

// resolveAgentWorkspace determines the workspace directory for an agent.
func resolveAgentWorkspace(agentCfg *config.AgentConfig, defaults *config.AgentDefaults) string {
	if agentCfg != nil && strings.TrimSpace(agentCfg.Workspace) != "" {
		return expandHome(strings.TrimSpace(agentCfg.Workspace))
	}
	if agentCfg == nil || agentCfg.Default || agentCfg.ID == "" || routing.NormalizeAgentID(agentCfg.ID) == "main" {
		return expandHome(defaults.Workspace)
	}
	home, _ := os.UserHomeDir()
	id := routing.NormalizeAgentID(agentCfg.ID)
	return filepath.Join(home, ".mobaiclaw", "workspace-"+id)
}

// resolveAgentModel resolves the primary model for an agent.
func resolveAgentModel(agentCfg *config.AgentConfig, defaults *config.AgentDefaults) string {
	if agentCfg != nil && agentCfg.Model != nil && strings.TrimSpace(agentCfg.Model.Primary) != "" {
		return strings.TrimSpace(agentCfg.Model.Primary)
	}
	return defaults.Model
}

// resolveAgentFallbacks resolves the fallback models for an agent.
func resolveAgentFallbacks(agentCfg *config.AgentConfig, defaults *config.AgentDefaults) []string {
	if agentCfg != nil && agentCfg.Model != nil && agentCfg.Model.Fallbacks != nil {
		return agentCfg.Model.Fallbacks
	}
	return defaults.ModelFallbacks
}

func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return home + path[1:]
		}
		return home
	}
	return path
}
