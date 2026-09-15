---
layout: home

hero:
  name: Stele
  text: Evidence for the behavior you specified
  tagline: Connect OpenSpec requirements to concrete implementation and independently executed tests through stable IDs and deterministic reports.
  actions:
    - theme: brand
      text: Install and verify
      link: /guide/getting-started
    - theme: alt
      text: Understand the model
      link: /concepts/openspec-and-stele

features:
  - icon: ID
    title: Behavior keeps its identity
    details: Requirement and scenario IDs survive wording, file, and implementation changes.
  - icon: ↗
    title: Plans resolve to real code
    details: Anchors must match the file and declaration named in the linkage plan.
  - icon: ✓
    title: Tests must actually run
    details: A successful process is insufficient. Stele confirms the exact named scenario test executed and passed.
  - icon: {}
    title: Reports stay deterministic
    details: Identical relevant inputs produce byte-for-byte identical JSON and stable exit codes.
---

## From prose to reviewable evidence

Stele adds an evidence layer to OpenSpec without introducing a second specification format.

<div class="evidence-flow">
  <div><strong>OpenSpec</strong><span>Requirements and concrete scenarios</span></div>
  <div><strong>Plan</strong><span>Expected code and named tests</span></div>
  <div><strong>Anchors</strong><span>IDs beside real declarations</span></div>
  <div><strong>Execution</strong><span>Exact scenario tests run independently</span></div>
  <div><strong>Evidence</strong><span>Revision-bound JSON for CI and review</span></div>
</div>

```bash
npm install --save-dev stele-spec
npx stele init --change account-recovery
npx stele verify --stage proposal
npx stele validate --json
```

::: info Current scope
The package ships a Go verification executable, an OpenSpec adapter, and repository-local planning and verification skills. The generated HTML report and interactive local dashboard are the next presentation layer described in the [dashboard roadmap](/roadmap/dashboard).
:::
