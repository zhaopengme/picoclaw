# MobaiClaw Built-in Skills Replacement Design

## Overview

This document outlines the strategic plan for replacing the built-in skills of the `MobaiClaw` project with the user's personal skills directory. The goal is to establish the user's current local skills (`~/.picoclaw/workspace/skills/`) as the official default templates for new installations, ensuring all branding remains consistent with "MobaiClaw".

## Motivation

The project currently ships with a set of default skills (e.g., `skill-creator`, `hardware`). However, the user has developed a richer, customized set of skills locally (e.g., `skill-builder`, `mcporter`, `thinking-partner`). To align the project's out-of-the-box experience with the user's latest workflows, a complete overwrite is required.

## Approach: Full Overwrite & Rebrand (Approach 1)

We will execute a "scorched earth" replacement strategy to guarantee no stale files or hybrid states remain.

### 1. Preparation & Clearing
- Target Directory: `cmd/mobaiclaw/workspace/skills/`
- Action: Completely remove the existing `skills` directory and all its contents within the project repository.

### 2. Copying Source of Truth
- Source Directory: `~/.picoclaw/workspace/skills/`
- Action: Recursively copy the entire contents of the source directory into the newly cleared target directory in the repository.

### 3. Deep Rebranding (The "MobaiClaw" Pass)
Since the source files originate from the `~/.picoclaw` directory, they inherently contain outdated branding ("picoclaw"). We must execute a deep find-and-replace across all newly copied `.md` and script files within `cmd/mobaiclaw/workspace/skills/`.

**Transformations required:**
- `PicoClaw` -> `MobaiClaw` (Case-sensitive, for display names/titles)
- `picoclaw` -> `mobaiclaw` (Case-sensitive, for file paths, commands, module names)
- `PICOCLAW` -> `MOBAICLAW` (Case-sensitive, for environment variables or constants)

### 4. Verification & Commit
- Verification: Run a recursive `grep` case-insensitively for "picoclaw" inside `cmd/mobaiclaw/workspace/skills/`. The expected output is strictly empty.
- Version Control: Stage the deleted, added, and modified files in the `skills/` directory and commit them with a descriptive message like `feat: overwrite built-in skills with user local skills and rebrand`.

## Conclusion

This approach guarantees that the project's embedded templates perfectly mirror the user's highly-developed local skill set, seamlessly integrating them into the MobaiClaw ecosystem without leaving any trace of the legacy branding.
