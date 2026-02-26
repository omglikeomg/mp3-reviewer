# Task 1: Project Setup & Input Adapters

## Context
We need to map the user's specific JSON structure into a standardized internal `Task` format.

## Implementation Details
1. **Domain Model (`/internal/domain/models.go`):**
   - Create `Task` struct: `FilePath`, `Title`, `Artist`, `Album`, `Genre1`, `Genre2`, `Year`, `BPM`.
   - Create `AppConfig` struct: `MusicFolder`, `JsonPath`, `GenreList []string`.
2. **JSON Adapter (`/internal/provider/`):**
   - Define an interface `TaskProvider` with a method `GetTasks() ([]Task, error)`.
   - Implement `ManualReviewProvider` that specifically parses the user's JSON schema (the `manual_review` array).
   - Map `filepath` to the full system path by joining it with the `MusicFolder` config.
3. **Queue Logic:**
   - Create a `ReviewQueue` in `/internal/domain` to track `CurrentIndex` and a `History []Task` for the Undo feature.

## Agent Checklist
- [ ] Implement `domain/models.go`
- [ ] Implement `provider/json_provider.go`
- [ ] Add a unit test in `provider/json_provider_test.go` using the user's sample JSON snippet.
- [ ] Update `architecture-breakdown.md` if the provider interface changes.
- [ ] Update `README.md` with instructions on how to structure the `settings.json` and the expected JSON file for manual review.
