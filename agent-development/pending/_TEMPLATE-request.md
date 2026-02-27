# Task <N>: <Short Descriptive Title>

## Goal

<!-- One or two sentences describing what this task achieves. What is the end state? -->

## Context

<!-- Why is this task needed? What problem does it solve? Reference any prerequisite tasks by number (e.g., "Requires Task 2 to be completed first"). Explain how this fits into the broader application architecture. -->

## Requirements

<!-- A bulleted list of concrete requirements. Each requirement should be verifiable — the implementing agent (or reviewer) should be able to confirm whether it was met or not. -->

- Requirement 1
- Requirement 2
- Requirement 3

## Implementation Details

<!-- Detailed technical guidance for the implementing agent. Include: -->
<!-- - Which files to create or modify (with full paths from project root) -->
<!-- - Struct definitions, function signatures, or interface contracts -->
<!-- - Algorithmic notes or edge cases to handle -->
<!-- - Any concurrency or error-handling considerations -->

1. **Component Name (`/path/to/file.go`):**
   - Detail 1
   - Detail 2

## Deliverables

<!-- A checklist of tangible outputs. What files, features, or artifacts must exist when this task is done? -->

- [ ] Deliverable 1
- [ ] Deliverable 2
- [ ] Deliverable 3

## Agent Checklist

<!-- Verification steps the implementing agent must complete before marking this task as done. These should be concrete, runnable checks — not vague affirmations. -->

- [ ] Check 1 (e.g., `go build ./...` succeeds with zero errors)
- [ ] Check 2 (e.g., unit tests pass: `go test ./internal/provider/...`)
- [ ] Check 3 (e.g., `README.md` updated with new configuration options)
- [ ] Update `agent-development/agent-specs/architecture-breakdown.md` if new packages or interfaces were introduced.
- [ ] Update `agent-development/agent-specs/FOLDER-STRUCTURE.md` if new packages or top-level directories were introduced.
- [ ] Update `README.md` with latest considerations.