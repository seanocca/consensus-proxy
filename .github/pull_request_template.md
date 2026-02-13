## NOTE
**Delete this section once complete.**

PR title must meet semantic release format and include a GitHub issue reference or NOSTORY plus a quick description. PRs should be squash merged only, using the PR title as the commit message.

### Example commit messages
- feat: #123: Short Description
- fix(optional-scope): NOSTORY: Short Description

Breaking changes (!) are not allowed. A major version change requires manual intervention.

### The following commit types are allowed

| Type | Version | Usage |
| :---- | :---- | :---- |
| - | major | A major version change requires manual intervention.
| feat | minor | A new feature was added.
| revert | minor | A commit was reverted.
| sec | minor | A security issue was fixed.
| perf | minor | A performance issue was fixed.
| fix | patch | A generic bugfix was made.
| refactor | patch | Code was refactored but otherwise unchanged expected behaviour.
| test | patch | Test code (only) was updated.
| chore | patch | A dependency was updated (or similar)
| build | patch | Changes were made to how builds are executed
| style | patch | Linting or other generic non-functional code changes.
| docs | none | Documentation was updated.
| ci | none | Changes made to how CI/CD is executed with no impact on the final build.
| wip | none | Catch-all for anything else. Will not appear in future release notes.


## Description
(Please include a summary of the change and which issue is fixed. Please also include relevant motivation and context. List any dependencies that are required for this change.)

## How Has This Been Tested?
(Please describe the tests that you ran to verify your changes. Provide instructions so we can reproduce. Please also list any relevant details for your test configuration.)