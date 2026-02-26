# Small Code Audit: Implementing a Safe Write Strategy for JSON Persistence

Perform a Code Audit to ensure that if the user hits Ctrl-C mid-write, the JSON file doesn't get corrupted.

Implement a 'Write to Temporary File and Rename' strategy for the JSON persistence.
