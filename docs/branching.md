# Branching and review flow

## Branches
- `main`: released, always deployable. Protected. Changes arrive only by pull request from `develop`.
- `develop`: integration branch. Protected (pull request and passing CI required). Changes arrive only by pull request from feature branches.
- `feature/<story-id>-<short-name>`: one branch per story, always cut from the latest `develop`.
  Examples: `feature/003-server-skeleton`, `feature/004-web-map-list`.
  Not tied to a story (docs, CI, chores): `feature/chore-<short-name>`.
  Bug found after a release: `feature/fix-<short-name>` from `main`, PR to `main`, then merge `main` back into `develop`.

`<story-id>` is the three-digit number of the file in `docs/stories/`. `<short-name>` is 2-4 lowercase words joined by hyphens.

## Commits
- One short plain sentence in the imperative describing the change, for example `Add healthz endpoint with db check`. No prefixes or ticket numbers needed (the branch carries the story id).
- Sign off every commit (`git commit -s`, DCO). Keep commits small; several per story is normal.

## Who does what
| Step | Who |
|---|---|
| Create the branch from `develop`, commit as work progresses | dev agent |
| Run local checks (`make test`, `make db-reset` when SQL changed) | dev agent |
| Push the branch and open the PR into `develop` | human (agents never push) |
| Check the acceptance criteria, write the test report | qa agent |
| Read-only review, comments on the PR | reviewer agent |
| Merge (squash merge) once CI is green and review issues are fixed | human |
| Release: PR `develop` -> `main` (merge commit), tag `vX.Y.Z` | human, at sprint end |

Max two fix rounds per story; after that the PM (chat) re-scopes the story.

## Pull requests
- Title: story id and name, for example `003 Server skeleton`.
- Body: link to the story file, the dev agent's report (under 200 words), anything not done.
- Keep PRs to one story. Rebase on `develop` before opening if it moved.

## Branch protection (GitHub settings)
`main`: require a pull request, require the CI status check `Migrations, OpenAPI/config validation, Go tests`, require branches up to date, block force pushes and deletion, include administrators.
`develop`: same, with squash merge only. Required approvals can stay at 0 while the maintainer is the only human; the reviewer agent's comments are the review.
Delete feature branches after merge.
