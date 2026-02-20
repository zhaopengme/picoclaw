# MobaiClaw Rebrand Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rebrand the entire project from "picoclaw" to "mobaiclaw" and update the module path to `github.com/zhaopengme/mobaiclaw`.

**Architecture:** We will use automated shell commands (`sed`, `mv`) to perform a deep rename. The process is split into renaming the module path in Go source code, updating configuration and command names, renaming directories, and finally updating all documentation and build scripts.

**Tech Stack:** Bash (`sed`, `find`, `mv`), Go

---

### Task 1: Update Go Module and Import Paths

**Files:**
- Modify: `go.mod`
- Modify: All `*.go` files in `pkg/` and `cmd/`

**Step 1: Write the failing test / Verify broken state**

Run: `go list -m`
Expected: `github.com/sipeed/picoclaw`

**Step 2: Implement the minimal code (Module Rename)**

```bash
# Update go.mod
go mod edit -module github.com/zhaopengme/mobaiclaw

# Update all import paths in Go source files
find . -name "*.go" -type f -exec sed -i '' 's|github.com/sipeed/picoclaw|github.com/zhaopengme/mobaiclaw|g' {} +
```
*(Note for Linux implementers: Remove `''` after `sed -i` if not on macOS, but macOS is assumed here based on environment).*

**Step 3: Run test to verify it passes**

Run: `go mod tidy && go test ./...`
Expected: PASS (All tests should pass since we only changed import paths consistently).

**Step 4: Commit**

```bash
git add -u
git commit -m "refactor: rename go module path to github.com/zhaopengme/mobaiclaw"
```

---

### Task 2: Rename Config Paths and Application Identity in Code

**Files:**
- Modify: `cmd/picoclaw/*.go`
- Modify: `pkg/**/*.go`

**Step 1: Identify targets**

Run: `grep -rin "picoclaw" pkg/ cmd/`
Expected: Output showing `.picoclaw` paths, `PICOCLAW_` env vars, and logging prefixes.

**Step 2: Implement the minimal code (Identity Rename)**

Replace occurrences of `.picoclaw` with `.mobaiclaw`, `picoclaw` with `mobaiclaw`, and `PicoClaw` with `MobaiClaw`.

```bash
# Case-sensitive replace for PicoClaw -> MobaiClaw
find pkg/ cmd/ -name "*.go" -type f -exec sed -i '' 's/PicoClaw/MobaiClaw/g' {} +

# Case-sensitive replace for picoclaw -> mobaiclaw
find pkg/ cmd/ -name "*.go" -type f -exec sed -i '' 's/picoclaw/mobaiclaw/g' {} +

# Uppercase replace for env vars PICOCLAW -> MOBAICLAW
find pkg/ cmd/ -name "*.go" -type f -exec sed -i '' 's/PICOCLAW/MOBAICLAW/g' {} +
```

**Step 3: Verify tests and run**

Run: `go build ./cmd/picoclaw && go test ./...`
Expected: PASS and build succeeds.

**Step 4: Commit**

```bash
git add -u
git commit -m "refactor: update application identity and config paths to mobaiclaw"
```

---

### Task 3: Rename Build Scripts and Directories

**Files:**
- Rename: `cmd/picoclaw` directory to `cmd/mobaiclaw`
- Modify: `Makefile`
- Modify: `.goreleaser.yaml`
- Modify: `Dockerfile`
- Modify: `Dockerfile.goreleaser`

**Step 1: Implement the minimal code (Rename and Update Scripts)**

```bash
# Rename the main command directory
mv cmd/picoclaw cmd/mobaiclaw

# Update Makefile
sed -i '' 's/picoclaw/mobaiclaw/g' Makefile
sed -i '' 's/PicoClaw/MobaiClaw/g' Makefile

# Update Dockerfiles
sed -i '' 's/picoclaw/mobaiclaw/g' Dockerfile
sed -i '' 's/picoclaw/mobaiclaw/g' Dockerfile.goreleaser
sed -i '' 's/picoclaw/mobaiclaw/g' docker-compose.yml

# Update Goreleaser
sed -i '' 's/picoclaw/mobaiclaw/g' .goreleaser.yaml
```

**Step 2: Verify build**

Run: `make build`
Expected: A binary named `mobaiclaw` (or `mobaiclaw-darwin-arm64` etc.) is generated in the `build/` directory without errors.

**Step 3: Commit**

```bash
git add cmd/ Makefile Dockerfile* docker-compose.yml .goreleaser.yaml
git commit -m "build: rename build scripts and directories to mobaiclaw"
```

---

### Task 4: Update Documentation and Workspace

**Files:**
- Modify: `README*.md`
- Modify: `ROADMAP.md`
- Modify: `workspace/*.md`

**Step 1: Implement the minimal code (Docs Rename)**

```bash
# Update READMEs and ROADMAP
find . -name "*.md" -type f -maxdepth 1 -exec sed -i '' 's/PicoClaw/MobaiClaw/g' {} +
find . -name "*.md" -type f -maxdepth 1 -exec sed -i '' 's/picoclaw/mobaiclaw/g' {} +
find . -name "*.md" -type f -maxdepth 1 -exec sed -i '' 's/sipeed/zhaopengme/g' {} +

# Update workspace templates
find workspace/ -name "*.md" -type f -exec sed -i '' 's/PicoClaw/MobaiClaw/g' {} +
find workspace/ -name "*.md" -type f -exec sed -i '' 's/picoclaw/mobaiclaw/g' {} +
```

**Step 2: Verify changes**

Run: `grep -i "picoclaw" README.md || echo "Clean"`
Expected: Output "Clean".

**Step 3: Commit**

```bash
git add README*.md ROADMAP.md workspace/
git commit -m "docs: rebrand all documentation and templates to MobaiClaw"
```