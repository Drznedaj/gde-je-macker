# Branching and review flow

## Branches
- `main`: the only long-lived branch. Always deployable. Protected: changes arrive only by pull request.
- `feature/<story-id>-<short-name>`: one branch per story, always cut from the latest `main`.
  Examples: `feature/003-server-skeleton`, `feature/004-web-map-list`.
  Not tied to a story (docs, CI, chores): `feature/chore-<short-name>`. Fixes: `feature/fix-<short-name>`.
- A separate dev/staging environment, when we have one, is deployed from a feature branch or a tag, not from a `develop` branch.

`<story-id>` is the three-digit number of the file in `docs/stories/`. `<short-name>` is 2-4 lowercase words joined by hyphens.

## Commits
- One short plain sentence in the imperative describing the change, for example `Add healthz endpoint with db check`. No prefixes or ticket numbers (the branch carries the story id).
- Sign off every commit (`git commit -s`, DCO). Keep commits small; several per story is normal.

## Who does what
| Step | Who |
|---|---|
| Create the branch from `main`, commit as work progresses | dev agent |
| Run local checks (`make test`, `make db-reset` when SQL changed) | dev agent |
| Push the branch and open the PR into `main` | human (agents never push) |
| Check the acceptance criteria, write the test report | qa agent |
| Read-only review, comments on the PR | reviewer agent |
| Squash merge once CI is green and review issues are fixed | human |

Max two fix rounds per story; after that the PM (chat) re-scopes the story. Releases are tags (`vX.Y.Z`) on `main`.

## Pull requests
- Title: story id and name, for example `003 Server skeleton`.
- Body: link to the story file, the dev agent's report (under 200 words), anything not done.
- One story per PR. Rebase on `main` before opening if it moved.

## Branch protection (GitHub ruleset on `main`)
Restrict deletions, require linear history, require a pull request (0 approvals while there is one human; squash merge only), require status check `Migrations, OpenAPI/config validation, Go tests` with branches up to date, block force pushes. Delete feature branches after merge.
