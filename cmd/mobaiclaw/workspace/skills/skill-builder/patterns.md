# Skill Patterns

Reference — load when implementing specific patterns.

## Pattern 1: Auto-Adaptive Skills

Skills that learn user preferences over time.

```
skill/
├── SKILL.md        ← Instructions + empty sections to fill
├── dimensions.md   ← What to detect (not in context by default)
└── criteria.md     ← When to confirm preferences
```

**SKILL.md structure:**
```markdown
## Auto-Adaptive [Domain]

This skill auto-evolves. Edit sections below as you learn.

**Rules:**
- Detect patterns from [signals]
- Confirm after 2+ consistent occurrences
- Check `dimensions.md` for categories
- Check `criteria.md` for format

---

### [Category 1]
<!-- Format: "key: value" -->

### [Category 2]
<!-- Format: "trait" -->

### Never
<!-- Things user rejected -->

---
*Empty = no preference yet. Observe and fill.*
```

## Pattern 2: Multi-Variant Skills

Skills supporting multiple frameworks/tools.

```
skill/
├── SKILL.md           ← Workflow + variant selection
└── references/
    ├── variant-a.md
    ├── variant-b.md
    └── variant-c.md
```

**SKILL.md structure:**
```markdown
## [Task]

### Variant Selection
- Use A when: [condition]
- Use B when: [condition]

### Workflow
1. Determine variant
2. Load relevant reference
3. Execute

For A details: see `references/variant-a.md`
```

## Pattern 3: Process Skills

Skills for multi-step procedures.

```markdown
## [Process Name]

### Steps
1. **[Step]** — [Brief description]
2. **[Step]** — [Brief description]

### Common Issues
- [Issue]: [Solution]

For detailed examples: see `examples.md`
```

## Pattern 4: Tool Integration Skills

Skills wrapping specific tools/APIs.

```markdown
## [Tool Name]

### Quick Commands
- `command` — what it does
- `command` — what it does

### Common Workflows
[Most frequent use case with example]

For API reference: see `reference.md`
For troubleshooting: see `troubleshooting.md`
```

## Good Description Examples

```yaml
description: "Build Flutter apps with clean architecture. Use for mobile app development, iOS/Android projects, or when user mentions Flutter."
```

```yaml
description: "Manage GitHub PRs, issues, and workflows. Triggers on: PR review, issue triage, CI/CD, repository management."
```

**Pattern:** What it does + explicit triggers/contexts.
