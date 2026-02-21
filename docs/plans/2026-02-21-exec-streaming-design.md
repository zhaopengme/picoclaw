# ExecTool Real-Time Streaming Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Provide real-time, streaming terminal output in the Telegram UI while long-running `exec` commands execute, replacing the static "Thinking... 💭" placeholder with a dynamic log view.

**Architecture:** Introduce a `ProgressTool` interface with a `ProgressCallback`. `ExecTool` will implement this, switching from `cmd.Run()` to reading `StdoutPipe`/`StderrPipe` in real-time. The `AgentLoop` will inject a debounced (rate-limited) callback that publishes `status_update` messages to the event bus, which the Telegram channel will render by updating the existing placeholder message.

**Tech Stack:** Go, internal event bus (`pkg/bus`), `os/exec` pipes, `bufio.Scanner`.

---

### Task 1: Define Progress Interfaces

**Files:**
- Modify: `pkg/tools/base.go`

**Step 1: Write minimal implementation**

In `pkg/tools/base.go`, add the `ProgressCallback` and `ProgressTool` interface:

```go
// ProgressCallback is a function type that tools can use to report real-time progress.
type ProgressCallback func(content string)

// ProgressTool is an optional interface that tools can implement to support
// reporting real-time progress during execution.
type ProgressTool interface {
	Tool
	SetProgressCallback(cb ProgressCallback)
}
```

**Step 2: Commit**

```bash
git add pkg/tools/base.go
git commit -m "feat(tools): define ProgressTool interface for real-time updates"
```

### Task 2: Support Progress in Registry

**Files:**
- Modify: `pkg/tools/registry.go`
- Modify: `pkg/tools/toolloop.go` (if it calls ExecuteWithContext, though usually only AgentLoop does directly. We will focus on `registry.go` and `AgentLoop`).

**Step 1: Write minimal implementation**

In `pkg/tools/registry.go`, update `ExecuteWithContext` to accept `progressCallback`:

```go
// ExecuteWithContext executes a tool with channel/chatID context, optional async callback, and optional progress callback.
func (r *ToolRegistry) ExecuteWithContext(ctx context.Context, name string, args map[string]interface{}, channel, chatID string, asyncCallback AsyncCallback, progressCallback ProgressCallback) *ToolResult {
```

Inside `ExecuteWithContext`, after setting contextual and async tools, inject the progress callback:

```go
	// If tool implements ProgressTool and callback is provided, set callback
	if progTool, ok := tool.(ProgressTool); ok && progressCallback != nil {
		progTool.SetProgressCallback(progressCallback)
		logger.DebugCF("tool", "Progress callback injected", map[string]interface{}{"tool": name})
	}
```

In `Execute`, pass `nil`:
```go
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]interface{}) *ToolResult {
	return r.ExecuteWithContext(ctx, name, args, "", "", nil, nil)
}
```

*Note: Update any other callers of `ExecuteWithContext` in the codebase to pass `nil` if necessary. (Check `pkg/tools/toolloop.go` or `pkg/agent/loop.go`).*

**Step 2: Commit**

```bash
git add pkg/tools/registry.go
git commit -m "feat(tools): support injecting ProgressCallback in ToolRegistry"
```

### Task 3: Implement Streaming in ExecTool

**Files:**
- Modify: `pkg/tools/shell.go`

**Step 1: Add Fields and Interface**

In `pkg/tools/shell.go`:
```go
type ExecTool struct {
	workingDir          string
	timeout             time.Duration
	denyPatterns        []*regexp.Regexp
	allowPatterns       []*regexp.Regexp
	restrictToWorkspace bool
	progressCb          ProgressCallback
}

func (t *ExecTool) SetProgressCallback(cb ProgressCallback) {
	t.progressCb = cb
}
```

**Step 2: Modify Execute to read streams**

In `Execute`, replace `cmd.Run()` with pipe reading:

```go
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to get stdout pipe: %v", err))
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to get stderr pipe: %v", err))
	}

	if err := cmd.Start(); err != nil {
		return ErrorResult(fmt.Sprintf("failed to start command: %v", err))
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var recentLogs []string
	var logsMu sync.Mutex

	// Helper to read pipe
	readPipe := func(pipe io.Reader, buf *bytes.Buffer, isErr bool) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			line := scanner.Text()
			buf.WriteString(line + "\n")

			if t.progressCb != nil {
				logsMu.Lock()
				recentLogs = append(recentLogs, line)
				if len(recentLogs) > 15 {
					recentLogs = recentLogs[len(recentLogs)-15:]
				}
				logStr := strings.Join(recentLogs, "\n")
				logsMu.Unlock()
				t.progressCb(logStr)
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); readPipe(stdoutPipe, &stdoutBuf, false) }()
	go func() { defer wg.Done(); readPipe(stderrPipe, &stderrBuf, true) }()

	wg.Wait()
	err = cmd.Wait()

	output := stdoutBuf.String()
	if stderrBuf.Len() > 0 {
		output += "\nSTDERR:\n" + stderrBuf.String()
	}
```
*(Ensure `bufio`, `io`, `sync` are imported).*

**Step 3: Commit**

```bash
git add pkg/tools/shell.go
git commit -m "feat(tools): implement real-time streaming in ExecTool"
```

### Task 4: Inject Debounced Callback in AgentLoop

**Files:**
- Modify: `pkg/agent/loop.go`

**Step 1: Write minimal implementation**

In `pkg/agent/loop.go`, inside `runLLMIteration` where `toolResult := agent.Tools.ExecuteWithContext(...)` is called (approx line 700):

Create the debounced callback:
```go
			// Create progress callback with debouncing
			var lastUpdate time.Time
			var mu sync.Mutex

			progressCallback := func(content string) {
				if constants.IsInternalChannel(opts.Channel) {
					return
				}

				mu.Lock()
				defer mu.Unlock()

				// Debounce: max 1 update every 2 seconds to avoid Telegram rate limits
				if time.Since(lastUpdate) > 2*time.Second {
					statusMsg := fmt.Sprintf("⚙️ 正在执行: %s...\n\n<pre>%s</pre>", tc.Name, content)
					al.bus.PublishOutbound(bus.OutboundMessage{
						Channel:  opts.Channel,
						ChatID:   opts.ChatID,
						Content:  statusMsg,
						Metadata: map[string]string{"status_update": "true"},
					})
					lastUpdate = time.Now()
				}
			}

			toolResult := agent.Tools.ExecuteWithContext(ctx, tc.Name, tc.Arguments, opts.Channel, opts.ChatID, asyncCallback, progressCallback)
```

Also, update the signature of `ExecuteWithContext` in any interface definitions if needed (e.g., if there's a mock or other usage).

**Step 2: Commit**

```bash
git add pkg/agent/loop.go
git commit -m "feat(agent): inject debounced progress callback for tool execution"
```
