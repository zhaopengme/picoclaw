# MobaiClaw Final Rebranding Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all remaining instances of "picoclaw" found during the final code review to complete the rebranding to MobaiClaw.

**Architecture:** We will use simple `sed` commands to replace strings in specific files that were missed by the initial batch replace, such as `.gitignore`, `.github` workflows, and `workspace` templates.

**Tech Stack:** Bash (`sed`, `grep`)

---

### Task 1: Update Agent Identity and Examples

**Files:**
- Modify: `cmd/mobaiclaw/workspace/IDENTITY.md`
- Modify: `cmd/mobaiclaw/workspace/SOUL.md`
- Modify: `cmd/mobaiclaw/workspace/skills/hardware/references/board-pinout.md`
- Modify: `config/config.example.json`

**Step 1: Write the failing test / Verify broken state**

Run: `grep -i "picoclaw" cmd/mobaiclaw/workspace/IDENTITY.md config/config.example.json`
Expected: Output showing instances of "picoclaw" or "PicoClaw".

**Step 2: Implement the minimal code (Identity Rename)**

```bash
# Update IDENTITY.md
sed -i '' 's/PicoClaw/MobaiClaw/g' cmd/mobaiclaw/workspace/IDENTITY.md
sed -i '' 's/picoclaw/mobaiclaw/g' cmd/mobaiclaw/workspace/IDENTITY.md
sed -i '' 's/sipeed/zhaopengme/g' cmd/mobaiclaw/workspace/IDENTITY.md

# Update SOUL.md
sed -i '' 's/picoclaw/mobaiclaw/g' cmd/mobaiclaw/workspace/SOUL.md

# Update board-pinout.md
sed -i '' 's/picoclaw/mobaiclaw/g' cmd/mobaiclaw/workspace/skills/hardware/references/board-pinout.md

# Update config.example.json
sed -i '' 's/picoclaw/mobaiclaw/g' config/config.example.json
```

**Step 3: Verify changes**

Run: `grep -i "picoclaw" cmd/mobaiclaw/workspace/IDENTITY.md cmd/mobaiclaw/workspace/SOUL.md cmd/mobaiclaw/workspace/skills/hardware/references/board-pinout.md config/config.example.json || echo "Clean"`
Expected: Output "Clean".

**Step 4: Commit**

```bash
git add cmd/mobaiclaw/workspace/ config/config.example.json
git commit -m "refactor: update agent identity and examples to MobaiClaw"
```

---

### Task 2: Update CI/CD, Issue Templates, and Ignores

**Files:**
- Modify: `.github/workflows/docker-build.yml`
- Modify: `.github/ISSUE_TEMPLATE/bug_report.md`
- Modify: `.gitignore`
- Modify: `.dockerignore`
- Modify: `LICENSE`

**Step 1: Verify broken state**

Run: `grep -i "picoclaw" .github/workflows/docker-build.yml .gitignore LICENSE`
Expected: Output showing old references.

**Step 2: Implement the minimal code (Infra Rename)**

```bash
# Update Docker build workflow
sed -i '' 's/picoclaw/mobaiclaw/g' .github/workflows/docker-build.yml

# Update Issue template
sed -i '' 's/PicoClaw/MobaiClaw/g' .github/ISSUE_TEMPLATE/bug_report.md

# Update gitignore
sed -i '' 's/picoclaw/mobaiclaw/g' .gitignore

# Update dockerignore
sed -i '' 's/picoclaw/mobaiclaw/g' .dockerignore

# Update LICENSE
sed -i '' 's/PicoClaw/MobaiClaw/g' LICENSE
```

**Step 3: Verify changes**

Run: `grep -i "picoclaw" .github/workflows/docker-build.yml .github/ISSUE_TEMPLATE/bug_report.md .gitignore .dockerignore LICENSE || echo "Clean"`
Expected: Output "Clean".

**Step 4: Commit**

```bash
git add .github/ .gitignore .dockerignore LICENSE
git commit -m "chore: update CI/CD, ignores, and license to MobaiClaw"
```