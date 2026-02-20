# Bootstrap Files Cleanup & Deprecation Design

## 1. Overview
This document outlines the final cleanup strategy for the `picoclaw` bootstrap workspace (`workspace/`). It details the deprecation and removal of static Markdown files that have been superseded by dynamic code-driven mechanisms, specifically the JSON memory system and dynamic tool registration.

## 2. Core Goal
Simplify the initialization workspace, reduce user cognitive load, and decrease LLM token consumption by removing redundant static configuration files.

## 3. Strategy by File

### A. Retained & Active (Core Context)
These files remain essential for defining the Agent's identity, behavior, and periodic tasks. They are actively loaded by `pkg/agent/context.go` or `pkg/heartbeat/service.go`.
*   `AGENTS.md`: Core directives and behavioral guidelines.
*   `IDENTITY.md`: Objective identity description (current state, environment, etc.).
*   `SOUL.md`: Persona and interaction tone.
*   `HEARTBEAT.md`: Periodic task prompts (read by the cron service).

### B. Deprecated & Removed
These files no longer serve a purpose in the current architecture and will be physically deleted from the `workspace/` source directory.

*   **`USER.md`**: Entirely replaced by the dynamic `profile.json` managed via `MemoryStore`.
    *   **Action:** Delete from `workspace/`.
    *   **Backwards Compatibility:** Relies on the previously implemented `NewMemoryStore().MigrateLegacyUserMD()` logic. If an old `USER.md` exists on a user's machine or is migrated from an older system, it will still be automatically converted to JSON.

*   **`TOOLS.md`**: Entirely replaced by the dynamic tool registration mechanism (`buildToolsSection` in `pkg/tools`). Tool availability and parameters are now 100% defined by Go code, and the LLM is only aware of registered tools.
    *   **Action:** Delete from `workspace/`.

## 4. Migration Tool Adjustments
Modifications required for the data migration logic from older versions or `openclaw` (`pkg/migrate/workspace.go`):
*   **Retain `USER.md`:** Must remain in the `migrateableFiles` list. This ensures old user preferences are copied over to trigger the `picoclaw` startup auto-conversion logic (`MigrateLegacyUserMD`).
*   **Remove `TOOLS.md`:** Must be removed from the `migrateableFiles` list. Even if migrated, the current `picoclaw` core logic does not read it, resulting in a dead file in the user's workspace.

## 5. Documentation Updates
*   Search and update all localized `README*.md` files.
*   Remove references to `USER.md` and `TOOLS.md` from the "Directory Structure" sections.
*   Emphasize in the documentation that user preferences are now automatically memorized in `memory/profile.json` and tool capabilities are built-in dynamically.