---
name: stele-archive
description: Archives a finished change the Stele way, only after its verification passes. Use when the user wants to archive or complete a change.
---

# Archive a change with Stele

1. Run `stele validate --change <change>`. If it fails, stop and report the failures.
2. Use the `openspec-archive-change` skill to archive the change.
3. Run `stele annotate --specs --targets-from openspec/changes/archive/<date>-<change>`, with the directory the archive just created. It restores the Stele annotation on the current specifications the archive created, and copies each capability's `targets` from the archived delta spec.
4. Run `stele validate --specs` and report the result.
