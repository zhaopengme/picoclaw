# MobaiClaw Rebrand Design

## 1. Overview
This document outlines the strategic plan for rebranding the `picoclaw` project to **`MobaiClaw`**. The rebranding involves a deep rename across the entire codebase to align with the `MobaiLabs` and user `zhaopengme` identities.

## 2. Core Identity Changes
*   **Old Name:** `PicoClaw` / `picoclaw`
*   **New Name:** `MobaiClaw` / `mobaiclaw`
*   **Old Go Module Path:** `github.com/sipeed/picoclaw`
*   **New Go Module Path:** `github.com/zhaopengme/mobaiclaw`
*   **Old User Config Dir:** `~/.picoclaw`
*   **New User Config Dir:** `~/.mobaiclaw`

## 3. Scope of Modifications

### A. Go Module and Import Paths
1.  Update `go.mod` to reflect `module github.com/zhaopengme/mobaiclaw`.
2.  Perform a global find-and-replace across all `.go` files in `pkg/` and `cmd/` to change import paths from `github.com/sipeed/picoclaw/` to `github.com/zhaopengme/mobaiclaw/`.

### B. Application Identity & Local Paths
1.  **CLI Command Name:** Change the primary command line entry point from `picoclaw` to `mobaiclaw`.
2.  **Configuration & Storage Paths:** Update the constants and configuration logic (e.g., `getGlobalConfigDir()`) to point to `.mobaiclaw` instead of `.picoclaw`.
3.  **Logs & Output:** Replace "PicoClaw" with "MobaiClaw" in CLI prompts, error messages, logging prefixes, and the ASCII logo display.

### C. Build Scripts & Environment
1.  **Makefile & Goreleaser:** Update `Makefile`, `.goreleaser.yaml`, and `Dockerfile` to build binaries named `mobaiclaw`.
2.  **Docker:** Rename image references (e.g., `sipeed/picoclaw` -> `zhaopengme/mobaiclaw` or `mobailabs/mobaiclaw`).
3.  **Environment Variables:** Refactor any environment variable prefixes from `PICOCLAW_` to `MOBAICLAW_`.

### D. Documentation & Marketing
1.  **READMEs:** Globally replace "PicoClaw" (case-sensitive and case-insensitive where appropriate) with "MobaiClaw" in `README.md` and all localized variants (`.zh.md`, `.ja.md`, etc.).
2.  **Repository Links:** Change references from `https://github.com/sipeed/picoclaw` to `https://github.com/zhaopengme/mobaiclaw` within the documentation.
3.  **Historical Comparisons:** Keep the historical structure intact (e.g., comparisons against "OpenClaw"), but update the subject to "MobaiClaw".

### E. Migration Tool Adjustments
1.  Update the CLI commands and prompts in `cmd_migrate.go` and `pkg/migrate/migrate.go` to guide users "Migrating from OpenClaw to MobaiClaw".
2.  Ensure target paths during migration point to the new `.mobaiclaw` structure.

## 4. Execution Strategy & Risks
*   **Automation:** Use `sed` or similar batch-processing tools to ensure consistent and comprehensive replacement across the vast codebase (~1000 occurrences).
*   **Testing:** After string replacements, run the entire test suite (`go test ./...`) to catch any broken import paths or string match assertions.
*   **External Dependency (User Action Required):** The GitHub repository **must** be renamed to `mobaiclaw` by the repository owner to prevent broken `go get` commands for future contributors or CI pipelines.