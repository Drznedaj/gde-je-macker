---
description: Run one story end to end: dev agent, PR, QA, review, auto-merge. Usage /story 003
---
Run story $ARGUMENTS following docs/branching.md. You are the orchestrator; keep your own output short and never paste code.

1. `git checkout main && git pull --ff-only`. Read docs/stories/$ARGUMENTS-*.md and pick its Owner agent. If the story lists a dependency that is not merged to main yet, stop and say so.
2. Run the owner agent on the story. It creates `feature/<id>-<short-name>` from main and commits locally. Require a passing local check (`make test`, plus `make db-reset` if SQL changed).
3. Write `docs/reports/<id>.md` (max 200 words: dev report, then later QA and reviewer verdicts with findings, then outcome) and commit it on the branch; keep updating it and pushing as rounds finish. The PM reads this file from the public repo, so no pasting is needed. Then `git push -u origin <branch>` then `gh pr create` (title `<id> <story name>`, body = story link + the agent's report).
4. Run the qa agent on the branch against the acceptance criteria, then the reviewer agent on `gh pr diff`. Each must end with `VERDICT: PASS` or `VERDICT: FAIL` plus a findings list. Post both as `gh pr comment`.
5. On FAIL, hand the findings to the owner agent, push, and repeat step 4. Max 2 fix rounds, then stop and report to the human.
6. Auto-merge only if ALL hold: both verdicts PASS, `gh pr checks` green, and the diff touches none of: `migrations/`, `.github/`, `.claude/`, `CLAUDE.md`, `config/radius.yaml`, `docs/adr/`, `LICENSE`, or code that handles manage tokens, photo storage or contact data. Then run `gh pr merge --auto --squash --delete-branch`.
7. Otherwise run `gh pr edit --add-label needs-human` (create the label if missing), leave the PR open, and tell the human why in two lines.
8. Finish with at most 5 lines: PR link, verdicts, merged or waiting, open questions.
