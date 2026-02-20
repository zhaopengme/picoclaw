# MobaiClaw Built-in Skills Replacement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Overwrite the project's built-in skills directory with the user's local `~/.picoclaw/workspace/skills/` directory and perform a deep rebrand to "MobaiClaw".

**Architecture:** We will delete the existing `cmd/mobaiclaw/workspace/skills/` directory, copy the user's local skills directory in its place, and execute a series of recursive `sed` commands to replace all instances of "PicoClaw", "picoclaw", and "PICOCLAW" with their "MobaiClaw" equivalents.

**Tech Stack:** Bash (`cp`, `rm`, `find`, `sed`, `grep`)

---

### Task 1: Replace and Rebrand Skills Directory

**Files:**
- Modify: `cmd/mobaiclaw/workspace/skills/` (entire directory)

**Step 1: Write the failing test / Verify broken state**

Run: `ls -la cmd/mobaiclaw/workspace/skills/skill-builder 2>/dev/null || echo "Missing skill-builder"`
Expected: Output showing "Missing skill-builder".

**Step 2: Implement the minimal code (Replace and Rebrand)**

```bash
# 1. Clear the target directory
rm -rf cmd/mobaiclaw/workspace/skills/*

# 2. Copy the user's local skills
cp -r ~/.picoclaw/workspace/skills/* cmd/mobaiclaw/workspace/skills/

# 3. Perform deep rebranding
# Case-sensitive replace for PicoClaw -> MobaiClaw
find cmd/mobaiclaw/workspace/skills/ -type f -exec sed -i '' 's/PicoClaw/MobaiClaw/g' {} +

# Case-sensitive replace for picoclaw -> mobaiclaw
find cmd/mobaiclaw/workspace/skills/ -type f -exec sed -i '' 's/picoclaw/mobaiclaw/g' {} +

# Uppercase replace for env vars PICOCLAW -> MOBAICLAW
find cmd/mobaiclaw/workspace/skills/ -type f -exec sed -i '' 's/PICOCLAW/MOBAICLAW/g' {} +
```

**Step 3: Verify changes**

Run: `grep -rin "picoclaw" cmd/mobaiclaw/workspace/skills/ || echo "Clean"`
Expected: Output "Clean" and no lines found.
Run: `ls -la cmd/mobaiclaw/workspace/skills/skill-builder 2>/dev/null && echo "Found skill-builder"`
Expected: Output showing the directory details and "Found skill-builder".

**Step 4: Commit**

```bash
git add cmd/mobaiclaw/workspace/skills/
git commit -m "feat: overwrite built-in skills with user local skills and rebrand to MobaiClaw"
```
