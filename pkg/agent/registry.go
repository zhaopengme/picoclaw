package agent

import (
	"fmt"
	"sync"

	"github.com/zhaopengme/mobaiclaw/pkg/bus"
	"github.com/zhaopengme/mobaiclaw/pkg/config"
	"github.com/zhaopengme/mobaiclaw/pkg/logger"
	"github.com/zhaopengme/mobaiclaw/pkg/providers"
	"github.com/zhaopengme/mobaiclaw/pkg/routing"
)

// AgentRegistry manages multiple agent instances and routes messages to them.
type AgentRegistry struct {
	agents    map[string]*AgentInstance
	resolver  *routing.RouteResolver
	mu        sync.RWMutex
	reloadMu  sync.Mutex // prevents concurrent reload operations
}

// NewAgentRegistry creates a registry from config, instantiating all agents.
func NewAgentRegistry(
	cfg *config.Config,
	provider providers.LLMProvider,
) *AgentRegistry {
	registry := &AgentRegistry{
		agents:   make(map[string]*AgentInstance),
		resolver: routing.NewRouteResolver(cfg),
	}

	agentConfigs := cfg.Agents.List
	if len(agentConfigs) == 0 {
		implicitAgent := &config.AgentConfig{
			ID:      "main",
			Default: true,
		}
		instance := NewAgentInstance(implicitAgent, &cfg.Agents.Defaults, cfg, provider)
		registry.agents["main"] = instance
		logger.InfoCF("agent", "Created implicit main agent (no agents.list configured)", nil)
	} else {
		for i := range agentConfigs {
			ac := &agentConfigs[i]
			id := routing.NormalizeAgentID(ac.ID)
			instance := NewAgentInstance(ac, &cfg.Agents.Defaults, cfg, provider)
			registry.agents[id] = instance
			logger.InfoCF("agent", "Registered agent",
				map[string]interface{}{
					"agent_id":  id,
					"name":      ac.Name,
					"workspace": instance.Workspace,
					"model":     instance.Model,
				})
		}
	}

	return registry
}

// GetAgent returns the agent instance for a given ID.
func (r *AgentRegistry) GetAgent(agentID string) (*AgentInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id := routing.NormalizeAgentID(agentID)
	agent, ok := r.agents[id]
	return agent, ok
}

// ResolveRoute determines which agent handles the message.
func (r *AgentRegistry) ResolveRoute(input routing.RouteInput) routing.ResolvedRoute {
	return r.resolver.ResolveRoute(input)
}

// ListAgentIDs returns all registered agent IDs.
func (r *AgentRegistry) ListAgentIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.agents))
	for id := range r.agents {
		ids = append(ids, id)
	}
	return ids
}

// CanSpawnSubagent checks if parentAgentID is allowed to spawn targetAgentID.
func (r *AgentRegistry) CanSpawnSubagent(parentAgentID, targetAgentID string) bool {
	parent, ok := r.GetAgent(parentAgentID)
	if !ok {
		return false
	}
	if parent.Subagents == nil || parent.Subagents.AllowAgents == nil {
		return false
	}
	targetNorm := routing.NormalizeAgentID(targetAgentID)
	for _, allowed := range parent.Subagents.AllowAgents {
		if allowed == "*" {
			return true
		}
		if routing.NormalizeAgentID(allowed) == targetNorm {
			return true
		}
	}
	return false
}

// GetDefaultAgent returns the default agent instance.
func (r *AgentRegistry) GetDefaultAgent() *AgentInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if agent, ok := r.agents["main"]; ok {
		return agent
	}
	for _, agent := range r.agents {
		return agent
	}
	return nil
}

// Reload rebuilds all agents from the new config and atomically swaps them in.
// The provider parameter is the newly created provider instance.
// setupFunc is called for each agent to register shared tools (can be nil).
// Returns error if new config validation fails.
func (r *AgentRegistry) Reload(cfg *config.Config, provider providers.LLMProvider, msgBus bus.Broker, setupFunc AgentSetupFunc) error {
	// Prevent concurrent reload operations
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()

	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	// Build new agents map
	newAgents := make(map[string]*AgentInstance)

	agentConfigs := cfg.Agents.List
	if len(agentConfigs) == 0 {
		// Create implicit main agent
		implicitAgent := &config.AgentConfig{
			ID:      "main",
			Default: true,
		}
		instance := NewAgentInstance(implicitAgent, &cfg.Agents.Defaults, cfg, provider)
		newAgents["main"] = instance
		logger.InfoCF("agent", "Reloaded implicit main agent (no agents.list configured)", nil)
	} else {
		for i := range agentConfigs {
			ac := &agentConfigs[i]
			id := routing.NormalizeAgentID(ac.ID)
			instance := NewAgentInstance(ac, &cfg.Agents.Defaults, cfg, provider)
			newAgents[id] = instance
			logger.InfoCF("agent", "Reloaded agent",
				map[string]interface{}{
					"agent_id":  id,
					"name":      ac.Name,
					"workspace": instance.Workspace,
					"model":     instance.Model,
				})
		}
	}

	// Call setup function for each agent (register shared tools)
	if setupFunc != nil {
		for id, instance := range newAgents {
			setupFunc(id, instance, cfg, msgBus, r, provider)
		}
	}

	// Build new resolver with new cfg (for bindings)
	newResolver := routing.NewRouteResolver(cfg)

	// Atomic swap
	r.mu.Lock()
	r.agents = newAgents
	r.resolver = newResolver
	r.mu.Unlock()

	logger.InfoCF("agent", "Registry reloaded",
		map[string]interface{}{
			"agent_count": len(newAgents),
		})

	return nil
}
