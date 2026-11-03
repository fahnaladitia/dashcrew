---
target: workflow/plan
order: 30
when: { has_knowledge: true }
rules:
  - plan.spec-named
  - plan.spec-acs
  - plan.spec-departures
  - plan.spec-revise-follow-up
---
### Spec

When step 0 names a spec (`Spec`, recorded in `git config branch.<branch>.dashcrew-spec`), read its `prd.md` and `trd.md` under `.ai/knowledge/specs/` before the File map, and add a Spec section:

- The spec and revision: `Spec: specs/<release>/<feature>@rev<n>`. When the clone's `rev` is higher, say so and plan against the newer one.
- The ACs this task covers, by ID, each in the verification map. `dashcrew spec status` lists the ACs other plans in this repo already claim.
- Every departure from the TRD, one line each: `departs from D-n: <what and why>`, or `C-n` for a contract. A change against a decision or contract without such a line makes the verdict NO-GO.
- A departure the user accepts needs a `spec revise` pull request on the knowledge repo, opened by a member; the summary names it as follow-up. Never edit an approved spec.
