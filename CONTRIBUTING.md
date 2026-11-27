# Contributing

Thanks for helping reunite pets with their families.

1. Open an issue describing the bug or idea before you start, so we can agree on scope.
2. Branch from `main` as `feature/fix-<short-name>` or `feature/chore-<short-name>` (see docs/branching.md), keep changes small and inside one area (`server`, `web`, `data`, `infra`, `mobile`).
3. Add or update tests. Run the checks listed in `CLAUDE.md`.
4. Sign off every commit (`git commit -s`). By signing off you certify the Developer Certificate of Origin (https://developercertificate.org). No CLA.
5. Open a pull request describing what changed and how you tested it.

## Ground rules
- Do not add personal data storage (accounts, emails, phone numbers in a user table, device locations).
- Do not add third-party trackers or paid services without an ADR in `docs/adr/`.
- Changes to `config/radius.yaml` must cite a source in the `evidence` field.
- Do not add third-party cartoon characters, artwork or logos.
