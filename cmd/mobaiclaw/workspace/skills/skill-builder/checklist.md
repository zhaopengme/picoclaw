# Pre-Publish Checklist

Run through before publishing any skill.

## Frontmatter

- [ ] `name` is clear and matches slug style
- [ ] `description` explains WHAT and WHEN (triggers)
- [ ] No extra fields in frontmatter

## SKILL.md Quality

- [ ] Under 80 lines (ideally 30-50)
- [ ] No walls of text
- [ ] Imperative voice throughout
- [ ] Every paragraph earns its tokens
- [ ] No "Introduction" or "About" sections
- [ ] No "When to use" in body (that's in description)

## Token Efficiency Test

For each section, ask:
- [ ] Does the agent really need this?
- [ ] Would the agent figure this out anyway?
- [ ] Is this actionable or just explanation?

**If answer is no → delete it.**

## Auxiliary Files

- [ ] Referenced from SKILL.md
- [ ] Clear when to load each one
- [ ] No duplicate content across files
- [ ] No deeply nested references

## Structure

- [ ] No README.md, CHANGELOG.md, etc.
- [ ] Scripts are tested and working
- [ ] Assets are necessary (not just nice-to-have)

## Final Test

Imagine the agent reading this skill:
- [ ] Would the agent know exactly what to do?
- [ ] Would the agent waste tokens re-reading for clarity?
- [ ] Is anything confusing or ambiguous?

## Common Mistakes

**Too much context:**
```markdown
❌ "Skills are modular packages that extend the agent's capabilities..."
✅ (Just start with what to do)
```

**Explaining the obvious:**
```markdown
❌ "YAML frontmatter is a way to add metadata..."
✅ (the agent knows what YAML is)
```

**Passive/verbose:**
```markdown
❌ "The user should be asked for their preferences"
✅ "Ask user for preferences"
```

**Body duplicating description:**
```markdown
❌ "Use this skill when creating new skills..."
✅ (Already in description, don't repeat)
```
