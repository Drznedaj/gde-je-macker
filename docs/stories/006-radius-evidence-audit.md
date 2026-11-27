# Story 006: Radius config evidence audit and density lookup design

Goal: confirm every value in config/radius.yaml has a sourced, honestly rated evidence note, and design the urban/rural density lookup.
Owner agent: data-radius
Files in scope: config/radius.yaml, data/**, docs/radius-notes.md (new)
Out of scope: Go code, API, frontend

## Acceptance criteria
- [ ] Each value has source (author, year, link), sample/region, confidence; Serbian/Balkan data noted if found, otherwise flagged as a gap.
- [ ] Dog evidence upgraded from secondary source to a primary study, or its confidence lowered.
- [ ] Proposal (not implementation) for classifying a point as urban/rural from open data (e.g. OSM/GHSL), with size and licence of the dataset, and a recommended simplest option.
- [ ] Changes to values are listed with reasons in the report; no silent edits.

## Report back (under 200 words)
Summary, changed values, open questions.
