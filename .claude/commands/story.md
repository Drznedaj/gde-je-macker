---
description: Run or resume one story end to end in this checkout or worktree: dev, PR, QA, review, sync with main, auto-merge. Usage /story 003
---
Run story $ARGUMENTS following `../pm/WORKFLOW.md` and docs/branching.md. You are the orchestrator; keep your own output short and never paste code.
Other stories may be running at the same time in sibling worktrees (`../wt-NNN`). Touch only this story's files in scope, stay on this story's branch, and never check out `main` (it is checked out in another worktree).

1. **Setup.** `git fetch origin`. Read `../pm/stories/$ARGUMENTS-*.md` and pick its Owner agent. For each story under "Depends on", check its PR is merged (`gh pr list --state merged --search "<id> in:title"`). If one is not merged, write `BLOCKED: waiting on <id> <PR link if open>` to `../pm/reports/$ARGUMENTS.md` and stop. This blocks only this story.
2. **Resume check.** If an open PR already exists for `feature/$ARGUMENTS-*` (`gh pr list --state open --search "head:feature/$ARGUMENTS-"`), switch to that branch and go to step 7.
3. **Branch.** Unless already on `feature/$ARGUMENTS-*`: `git switch -c feature/$ARGUMENTS-<short-name> origin/main`.
4. **Dev.** Run the owner agent on the story. It commits locally. Require a passing `make test` (plus `make db-reset` if SQL changed).
5. **Report and PR.** Write `../pm/reports/$ARGUMENTS.md` (max 200 words: dev report, later QA and reviewer verdicts, then outcome) and keep it updated. Never commit it. `git push -u origin <branch>`, then `gh pr create` (title `<id> <story name>`, body = the dev report).
6. **QA and review.** Run the qa agent against the acceptance criteria, then the reviewer agent on `gh pr diff`. Each ends with `VERDICT: PASS` or `VERDICT: FAIL` plus findings, posted with `gh pr comment` (plain comments, not review threads: unresolved threads block merging). On FAIL, hand findings to the owner agent, push, repeat. Max 2 fix rounds, then stop with `RESULT: FAILED`.
7. **Sync with main.** `git fetch origin`. If `git merge-base --is-ancestor origin/main HEAD` fails, run `git merge --no-edit origin/main` (never rebase; force-push is blocked).
   - Conflicts: hand them to the owner agent to resolve within this story's scope, run `make test`, commit, `git push origin <branch>`. Then re-run the reviewer on `gh pr diff` and post its verdict. If the conflict is in another story's files or the reviewer fails, add `needs-human` and comment why.
   - No conflicts: `git push origin <branch>`.
8. **Merge decision.**
   - A needs-human condition from `../pm/WORKFLOW.md` section 4 applies (diff touches `migrations/`, `.github/`, `.claude/`, `CLAUDE.md`, `config/radius.yaml`, `docs/adr/`, `LICENSE`, or code handling manage tokens, photo storage or contact/message data): `gh pr edit <n> --add-label needs-human` (create the label if missing), write why in the report, and finish. This is a normal end, not a failure: do not wait for the owner.
   - Otherwise, with both verdicts PASS: `gh pr merge <n> --auto --squash`. GitHub merges once the required check is green and the branch is up to date. Do not wait for it.
9. Never open doc-only PRs (stories, reports, spikes, ADR drafts). If the story produced no code change, stop after the report.
10. Finish with at most 5 lines. The last line is exactly one of: `RESULT: MERGING #<n>`, `RESULT: NEEDS-HUMAN #<n>`, `RESULT: BLOCKED <id>`, `RESULT: FAILED #<n>`. Write the same line at the end of the report.
