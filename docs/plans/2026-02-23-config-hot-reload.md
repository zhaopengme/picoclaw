# Config Hot-Reload Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable runtime reloading of agent configuration (agents.defaults.*, agents.list, bindings) via `/reload` command without restarting the gateway process. All active conversations should immediately use the new configuration.

**Strategy:** Full rebuild approach - on reload, completely recreate the AgentRegistry with new agent instances. Existing session history is preserved via disk-backed session stores.

**Scope (v1):**
- Hot-reloadable: `agents.defaults.*`, `agents.list`, `bindings`, `providers` (model_list)
- Non-reloadable (require restart): `channels.*`, `gateway.*`

---

## Overview

### Architecture Changes

```
┌─────────────────────────────────────────────────────────────────┐
│                     cmd_gateway.go                               │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  cfg, provider := loadConfig(), createProvider()          │   │
│  │                                                           │   │
│  │  agentLoop := agent.NewAgentLoop(cfg, agentBus, provider) │   │
│  │  ┌─────────────────────────────────────────────────────┐ │   │
│  │  │  registry := NewAgentRegistry(cfg, provider)        │ │   │
│  │  │  ┌───────────────────────────────────────────────┐  │ │   │
│  │  │  │  agents: map[string]*AgentInstance           │  │ │   │
│  │  │  │  resolver: *RouteResolver (holds cfg ref)    │  │ │   │
│  │  │  └───────────────────────────────────────────────┘  │ │   │
│  │  └─────────────────────────────────────────────────────┘ │   │
│  │                                                           │   │
│  │  gw := gateway.NewCommandGateway(...,                     │   │
│  │      agentLoop.GetRegistry(),                             │   │
│  │      reloadCallback)  // NEW: callback for /reload        │   │
│  │                                                           │   │
│  │  // On reload: both provider and registry are recreated   │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    pkg/gateway/gateway.go                        │
│                                                                  │
│  case "/reload":                                                 │
│    if g.reloadCallback != nil {                                 │
│      return g.reloadCallback(ctx, msg)  // trigger reload        │
│    }                                                             │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                   pkg/agent/registry.go                          │
│                                                                  │
│  func (r *AgentRegistry) Reload(                                │
│      cfg *config.Config,                                        │
│      provider providers.LLMProvider,                            │
│  ) error {                                                      │
│    // 1. Build new agents map from new cfg                      │
│    // 2. Build new resolver from new cfg (for bindings)         │
│    // 3. Atomic swap under RWMutex                              │
│  }                                                               │
└─────────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

1. **AgentRegistry owns reload logic** - The registry knows how to rebuild itself from config
2. **Atomic swap** - Use sync.RWMutex to ensure no reads during swap
3. **Provider IS reloaded** - On reload, provider is recreated via `providers.CreateProvider(newCfg)` and replaces the old instance
4. **Session preservation** - SessionManager instances are NOT recreated; they persist on disk
5. **Force switch** - All conversations immediately route to new agent configs
6. **Full rebuild** - Provider recreation triggers full AgentRegistry rebuild since all AgentInstance instances hold provider references

---

## Implementation Steps

### Step 1: Add Reload method to AgentRegistry

**File:** `/Users/zhaopeng/projects/github/mobaiclaw/pkg/agent/registry.go`

**Changes:**
```go
// Reload rebuilds all agents from the new config and atomically swaps them in.
// The provider parameter is the newly created provider instance.
// Returns error if new config validation fails.
func (r *AgentRegistry) Reload(cfg *config.Config, provider providers.LLMProvider) error {
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
    } else {
        for i := range agentConfigs {
            ac := &agentConfigs[i]
            id := routing.NormalizeAgentID(ac.ID)
            instance := NewAgentInstance(ac, &cfg.Agents.Defaults, cfg, provider)
            newAgents[id] = instance
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
```

**Commit:**
```bash
git add pkg/agent/registry.go
git commit -m "feat(agent): add Reload method to AgentRegistry for config hot-reload"
```

---

### Step 2: Create ReloadCallback in cmd_gateway.go

**File:** `/Users/zhaopeng/projects/github/mobaiclaw/cmd/mobaiclaw/cmd_gateway.go`

**Changes:**

1. Import `loadConfig` at package level (already exists via main.go, but need to make it accessible)

2. Create a reload callback function that:
   - Reloads config from disk
   - Validates new config
   - Creates a new provider instance
   - Calls registry.Reload() with new provider
   - Updates agentLoop's provider reference
   - Returns user-friendly message

Add after `setupCronTool` function:

```go
// createReloadCallback returns a function that handles /reload command.
// It reloads config from disk, recreates the provider, and updates the agent registry.
func createReloadCallback(agentLoop *agent.AgentLoop) func(ctx context.Context, msg bus.InboundMessage) (string, error) {
    return func(ctx context.Context, msg bus.InboundMessage) (string, error) {
        configPath := getConfigPath()

        // Load new config
        newCfg, err := config.LoadConfig(configPath)
        if err != nil {
            return fmt.Sprintf("Failed to load config: %v", err), nil
        }

        // Create new provider instance
        newProvider, modelID, err := providers.CreateProvider(newCfg)
        if err != nil {
            return fmt.Sprintf("Failed to create provider: %v", err), nil
        }

        // Trigger registry reload with new provider
        registry := agentLoop.GetRegistry()
        if err := registry.Reload(newCfg, newProvider); err != nil {
            return fmt.Sprintf("Failed to reload registry: %v", err), nil
        }

        // Update agentLoop's config and provider references
        agentLoop.UpdateConfig(newCfg)
        agentLoop.UpdateProvider(newProvider)

        // Get new agent count
        agentIDs := registry.ListAgentIDs()

        return fmt.Sprintf("Config reloaded successfully. Provider: %s, %d agent(s) available: %s",
            modelID, len(agentIDs), strings.Join(agentIDs, ", ")), nil
    }
}
```

3. Update `gatewayCmd` to wire up the callback:

Modify the gateway initialization (around line 188):

```go
// Before:
// gw := gateway.NewCommandGateway(mainBus, agentBus, channelManager, agentLoop.GetRegistry())

// After:
reloadCallback := createReloadCallback(agentLoop)
gw := gateway.NewCommandGateway(mainBus, agentBus, channelManager, agentLoop.GetRegistry(), reloadCallback)
```

**Commit:**
```bash
git add cmd/mobaiclaw/cmd_gateway.go
git commit -m "feat(gateway): wire up /reload command handler in cmd_gateway"
```

---

### Step 3: Add GetConfig, UpdateConfig, and UpdateProvider to AgentLoop

**File:** `/Users/zhaopeng/projects/github/mobaiclaw/pkg/agent/loop.go`

**Changes:**

```go
// GetConfig returns the current config.
func (al *AgentLoop) GetConfig() *config.Config {
    return al.cfg
}

// UpdateConfig updates the config reference.
func (al *AgentLoop) UpdateConfig(cfg *config.Config) {
    al.cfg = cfg
}

// UpdateConfig updates the provider reference.
func (al *AgentLoop) UpdateProvider(provider providers.LLMProvider) {
    al.provider = provider
}
```

**Commit:**
```bash
git add pkg/agent/loop.go
git commit -m "feat(agent): add GetConfig, UpdateConfig, and UpdateProvider to AgentLoop"
```

---

### Step 4: Update CommandGateway to support reload callback

**File:** `/Users/zhaopeng/projects/github/mobaiclaw/pkg/gateway/gateway.go`

**Changes:**

1. Add `reloadCallback` field to CommandGateway struct:

```go
type ReloadCallback func(ctx context.Context, msg bus.InboundMessage) (string, error)

type CommandGateway struct {
    bus            bus.Broker
    channelManager *channels.Manager
    agentBus       bus.Broker
    agentRegistry  *agent.AgentRegistry
    reloadCallback ReloadCallback // NEW
}
```

2. Update `NewCommandGateway`:

```go
func NewCommandGateway(b bus.Broker, agentBus bus.Broker, cm *channels.Manager, registry *agent.AgentRegistry, reloadCb ReloadCallback) *CommandGateway {
    return &CommandGateway{
        bus:            b,
        agentBus:       agentBus,
        channelManager: cm,
        agentRegistry:  registry,
        reloadCallback: reloadCb,
    }
}
```

3. Add `/reload` case in `handleCommand`:

```go
case "/reload":
    if g.reloadCallback == nil {
        return "Reload not available", true
    }
    // Call reload callback
    response, err := g.reloadCallback(ctx, msg)
    if err != nil {
        return fmt.Sprintf("Reload failed: %v", err), true
    }
    return response, true
```

4. Update `/help` output to include `/reload`:

```go
case "/help":
    return `/start - Start the bot
/clear - Clear current session history and summary
/reload - Reload agent configuration (agents.*, bindings, providers)
/help - Show this help message
/show [model|channel|agents] - Show current configuration
/list [models|channels|agents] - List available options
/switch [model|channel] to <name> - Switch current model or channel`, true
```

**Commit:**
```bash
git add pkg/gateway/gateway.go
git commit -m "feat(gateway): add /reload command support to CommandGateway"
```

---

## Testing Approach

### Manual Testing

1. **Test agents.defaults.* reload:**
   - Start gateway with initial config
   - Modify `agents.defaults.model` to a different model
   - Send `/reload` command
   - Verify new model is used for subsequent messages

2. **Test agents.list reload (add agent):**
   - Add new agent to `agents.list`
   - Send `/reload`
   - Send `/list agents` - new agent should appear
   - Route to new agent via binding

3. **Test agents.list reload (remove agent):**
   - Remove an agent from `agents.list`
   - Send `/reload`
   - Verify removed agent no longer appears in `/list agents`

4. **Test bindings reload:**
   - Add new binding for specific channel/peer
   - Send `/reload`
   - Send message from that channel/peer
   - Verify routing uses new binding

5. **Test invalid config:**
   - Corrupt config.json (invalid JSON)
   - Send `/reload`
   - Verify error message and original config still works

6. **Test empty config:**
   - Create empty config.json
   - Send `/reload`
   - Verify graceful degradation (implicit main agent)

7. **Test session preservation:**
   - Start conversation, accumulate history
   - Modify agent config (change model, add skills)
   - Send `/reload`
   - Continue conversation - history should be preserved

8. **Test provider reload (API key change):**
   - Start with provider config using API key A
   - Modify provider config to use API key B
   - Send `/reload`
   - Verify new provider is used for subsequent requests
   - Verify model list is updated if changed

9. **Test provider reload failure:**
   - Modify provider config with invalid API key
   - Send `/reload`
   - Verify error message and original provider still works

### Unit Tests

Add to `/Users/zhaopeng/projects/github/mobaiclaw/pkg/agent/registry_test.go`:

```go
func TestAgentRegistryReload(t *testing.T) {
    // Test 1: Reload with different agent list
    // Test 2: Reload preserves nothing (full rebuild)
    // Test 3: Reload with empty agent list (implicit main)
}

func TestAgentRegistryReloadConcurrency(t *testing.T) {
    // Test concurrent reads during reload
}
```

---

## Edge Cases to Handle

### 1. Invalid Config During Reload

**Behavior:** Return error message, keep existing config active

```go
if err := config.LoadConfig(path); err != nil {
    return fmt.Sprintf("Config reload failed: %v. Using previous config.", err), nil
}
```

### 2. Empty Config (No agents.list)

**Behavior:** Create implicit "main" agent (same as startup)

```go
if len(agentConfigs) == 0 {
    // Create implicit main agent
    ...
}
```

### 3. Provider API Key Change

**Behavior:** New provider is created with updated credentials

```go
// This is now supported - provider is recreated on reload
newProvider, modelID, err := providers.CreateProvider(newCfg)
```

**Edge case:** If provider creation fails (e.g., invalid API key), the reload is aborted and old config/provider remain active.

```go
if err != nil {
    return fmt.Sprintf("Failed to create provider: %v", err), nil
    // Old provider and registry remain active
}
```

### 4. Provider Type Change (e.g., OpenAI -> Anthropic)

**Behavior:** Handled by provider recreation. All AgentInstance instances receive the new provider via full rebuild.

### 5. Concurrent Requests During Reload

**Behavior:** RWMutex ensures readers wait for swap to complete

### 6. Missing Agent in Binding

**Behavior:** RouteResolver's `pickAgentID` already handles missing agents by falling back to default

### 7. Workspace Directory Changes

**Behavior:** New agent instances will use new workspace paths. Session history from old workspace won't be available (this is acceptable - sessions are per-workspace)

### 8. Removed Agent With Active Sessions

**Behavior:** Agent instance is garbage collected. Session files on disk remain but become inaccessible (acceptable - admin should archive if needed)

---

## Non-Goals (v1)

1. **Channel reload** - Requires stopping/starting HTTP servers, too invasive
2. **Graceful session migration** - When workspace changes, old sessions orphaned
3. **Config validation UI** - Errors returned as plain text
4. **Automatic reload on file change** - Manual trigger only

---

## Future Enhancements

1. **v2: Graceful session migration** - Copy session files when workspace changes
2. **Auto-reload** - Watch config file for changes (fsnotify)
3. **Diff reload** - Only recreate changed agents (optimization)
4. **Rollback** - `/reload --rollback` to revert to previous config
5. **Dry-run** - `/reload --dry-run` to validate without applying
6. **Provider connection pooling** - Reuse connections across provider recreation
