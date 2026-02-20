# Skill Structure

Reference — load when planning skill architecture.

## Required: SKILL.md

```yaml
---
name: "Skill Name"
description: "What it does. When to use it. Triggers."
---
```

**Body guidelines:**
- 30-50 lines ideal, 80 max
- Imperative voice ("Do X" not "You should do X")
- Reference auxiliary files, don't duplicate content
- No "About" or "Introduction" sections

## Optional: Auxiliary Files

**When to split out:**
- Content exceeds 50 lines
- Details only needed sometimes
- Multiple variants/frameworks
- Reference material

**Naming:**
- `[topic].md` — Domain-specific details
- `patterns.md` — Common patterns and examples
- `checklist.md` — Validation before use
- `criteria.md` — Decision criteria

**Reference from SKILL.md:**
```markdown
For deployment patterns, see `deploy.md`
```

## Optional: Scripts (`scripts/`)

Executable code for repetitive/deterministic tasks.

- Must be tested before including
- Include only if saves significant tokens
- Can be executed without loading into context

## Optional: Assets (`assets/`)

Files used in output, not loaded into context.

- Templates, images, boilerplate
- the agent uses them, doesn't read them

## Anti-patterns

❌ README.md, INSTALLATION.md, CHANGELOG.md
❌ "When to use this skill" in body (goes in description)
❌ Explanations of concepts the agent knows
❌ Deeply nested file references
❌ Duplicate info across files
