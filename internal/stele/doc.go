// Package stele implements deterministic traceability between OpenSpec behavior,
// source declarations, and exact test executions.
//
// OpenSpec requirement and scenario IDs are canonical. Source and test anchors
// reference those IDs, while reports keep linkage, execution, and human review as
// separate states. Given the same repository inputs, Stele emits the same ordered
// diagnostics and schema-compatible JSON evidence.
package stele
