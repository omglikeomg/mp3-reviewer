# Implementation Plan: Task <N> — <Short Descriptive Title>

## Overview

<!-- A brief paragraph summarizing what this plan accomplishes. State the goal, the scope, and any important constraints. This should give an implementing agent a clear mental model before diving into the steps. -->

---

## Reference Documents

Before starting, the implementing agent **must** read and internalize these files:

| Document | Path | Purpose |
|---|---|---|
| Application Overview | `agent-specs/application-overview.md` | Understand what the tool does |
| Architecture Breakdown | `agent-specs/architecture-breakdown.md` | Understand folder structure, design patterns, tech stack |
| Agent Instructions | `agent-specs/agent-instructions.md` | Coding standards, dos/don'ts, workflow |
| Task Definition | `agent-development/pending/<N>-<name>.md` | The task being implemented |
<!-- Add any other relevant reference files (existing source files, previous plans, etc.) -->

---

## Pre-Conditions

<!-- List everything that must be true before this plan can be executed. Examples: -->
<!-- - Which previous tasks must be completed? -->
<!-- - What files or directories must already exist? -->
<!-- - What tools or dependencies must be installed? -->

- Pre-condition 1
- Pre-condition 2

---

## Step-by-Step Implementation

### Step 1: <Action Title>

**Action:** <!-- What the agent must do — be specific. Include shell commands, file paths, and exact content where applicable. -->

**Expected outcome:** <!-- What the result should look like after this step. -->

**Verification:** <!-- How to confirm this step succeeded. Include a runnable command if possible. -->

---

### Step 2: <Action Title>

**Action:** <!-- ... -->

**Expected outcome:** <!-- ... -->

**Verification:** <!-- ... -->

---

<!-- Repeat for as many steps as needed. Each step should be atomic — one clear action with one verifiable outcome. -->

---

## File Manifest

Summary of every file created or modified in this task:

| # | File Path | Action | Content Summary |
|---|---|---|---|
| 1 | `path/to/file` | Created / Modified | Brief description of what's in the file |
<!-- Action should be "Created" for new files or "Modified" for changes to existing files. -->

**Total files created: X | Total files modified: Y**

---

## Post-Completion Checklist

The implementing agent must verify each item before marking this task as done:

- [ ] Verification item 1 (e.g., `go build ./...` succeeds with zero errors)
- [ ] Verification item 2 (e.g., `go test ./...` passes)
- [ ] Verification item 3 (e.g., specific file exists and contains expected content)
- [ ] No unrelated files were modified or deleted
- [ ] `architecture-breakdown.md` updated if new packages or interfaces were introduced
- [ ] `README.md` updated if user-facing behavior changed

---

## Notes for the Implementing Agent

<!-- Any additional guidance, warnings, or clarifications that don't fit into the steps above. Examples: -->
<!-- - Things the agent should NOT do -->
<!-- - Edge cases to be aware of -->
<!-- - Explanations of non-obvious design decisions -->

1. Note 1
2. Note 2