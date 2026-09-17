---
layout: home

hero:
  name: Stele
  text: Maintain software in plain English
  tagline: Describe how a codebase should behave, then deterministically verify that each behavior stays connected to implemented code and current evidence.
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
    title: People approve the evidence
    details: Each scenario's planned test levels are approved before implementation, and anchors in the code show where the evidence lives.
  - icon: ✓
    title: Tests must actually run
    details: A successful process is insufficient. Stele confirms the exact named scenario test executed and passed.
  - icon: {}
    title: Reports stay deterministic
    details: Identical relevant inputs produce byte-for-byte identical JSON and stable exit codes.
---

## From prose to reviewable evidence

Stele turns plain-English software behavior into a maintainable evidence graph. [OpenSpec](https://github.com/Fission-AI/OpenSpec) is the first specification foundation, so teams keep requirements and scenarios in its native workflow while Stele adds traceability and deterministic validation without introducing a second behavioral source of truth.

<div class="evidence-flow">
  <div><strong>OpenSpec</strong><span>Requirements and concrete scenarios</span></div>
  <div><strong>Plan</strong><span>Expected code and named tests</span></div>
  <div><strong>Anchors</strong><span>IDs beside real declarations</span></div>
  <div><strong>Execution</strong><span>Exact scenario tests run independently</span></div>
  <div><strong>Evidence</strong><span>Revision-bound JSON for CI and review</span></div>
</div>

```bash
npm install --save-dev stele-spec@next
npx stele init
# Ask your agent: "Use stele-propose to plan <your change>."
npx stele validate --change <your-change>
```

Read the [product intent](/intent/vision) for where Stele is heading.

::: info Current scope
The package ships a Go verification executable, TypeScript and Go consumer support, an OpenSpec adapter with a Stele workflow schema, and repository-local skills to propose, apply, archive, plan, and verify changes.
:::
