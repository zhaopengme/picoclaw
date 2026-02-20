---
name: skill-builder
description: "Build high-quality skills with optimal structure. For local use or publishing to ClawHub."
---

## Build Skills That Work

Create skills that are token-efficient, well-structured, and actually useful.

**Core principle:** SKILL.md should be SHORT. Move details to auxiliary files.

**References:**
- `structure.md` — Anatomy of a good skill
- `patterns.md` — Proven patterns with examples  
- `checklist.md` — Pre-publish validation

---

### File Structure

```
skill-name/
├── SKILL.md      ← Short! (~30-50 lines)
├── [topic].md    ← Details, loaded when needed
└── FILES.txt     ← Lists all files to publish (one per line)
```

**FILES.txt example:**
```
SKILL.md
patterns.md
reference.md
```

### ⚠️ Publishing Rules

**Publishing is serious.** Each publish is permanent and public. No spam, no experiments.

1. **Never publish without verification**
2. **Never change slug after first publish** (breaks installations)
3. **Always verify name, description, version match your files**
4. **Always confirm which files will be published**

### Pre-Publish Verification

Before ANY publish, send verification to user with:
- **Slug** (exact)
- **Name** (exact)  
- **Version**
- **Description** (exact)
- **Files list** (from FILES.txt)

**Recommended:** Generate verification programmatically (script/code) to guarantee accuracy. A PDF or structured format that pulls values directly from your files ensures no copy-paste errors.

User can configure verification level:
- Full review (PDF with all content)
- Summary only (metadata + file list)
- Trust mode (auto-publish, not recommended)

### What Makes Skills Good

- **Concise** — Models are smart, don't over-explain
- **Progressive disclosure** — Details in separate files
- **Clear triggers** — Description says WHEN to use
- **Actionable** — Instructions, not explanations

---

**Before publishing:** Run `checklist.md`
