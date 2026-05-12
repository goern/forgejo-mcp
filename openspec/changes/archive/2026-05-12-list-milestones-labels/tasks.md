## Tasks

### 1. Add list_repo_milestones tool

- [ ] 1.1 Add `ListRepoMilestonesToolName = "list_repo_milestones"` constant in `operation/issue/issue.go`
- [ ] 1.2 Define the tool with `mcp.NewTool(...)` including owner, repo, page, limit, state parameters
- [ ] 1.3 Implement `ListRepoMilestonesFn` handler calling `client.ListRepoMilestones`
- [ ] 1.4 Register the tool and handler in `RegisterTool`
- [ ] 1.5 Add test coverage in `test/` following existing test patterns

### 2. Add list_repo_labels tool

- [ ] 2.1 Add `ListRepoLabelsToolName = "list_repo_labels"` constant in `operation/issue/issue.go`
- [ ] 2.2 Define the tool with `mcp.NewTool(...)` including owner, repo, page, limit parameters
- [ ] 2.3 Implement `ListRepoLabelsFn` handler calling `client.ListRepoLabels`
- [ ] 2.4 Register the tool and handler in `RegisterTool`
- [ ] 2.5 Add test coverage in `test/` following existing test patterns

### 3. Documentation

- [ ] 3.1 Update `README.md` to list both new tools
- [ ] 3.2 Update `CHANGELOG.md` with entry for the new tools
