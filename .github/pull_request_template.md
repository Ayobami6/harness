## Summary of Changes

<!-- Briefly describe what this PR changes and why. Link the issue it resolves, e.g. Closes #123 -->

Closes #

## Architecture Decisions & Trade-offs

<!-- Document any schema changes, concurrency considerations, or design trade-offs. Write "N/A" if none. -->

## Verification Evidence

<!-- Paste the test and linter commands run and their output. -->

- [ ] `make lint` (`golangci-lint run`) passes with 0 issues
- [ ] `make test` (`go test -race -v ./...`) passes
- [ ] Database migrations (`make migrate-up` / `make migrate-down`) verified (if applicable)
- [ ] `make generate` (`sqlc generate`) run and committed (if applicable)

