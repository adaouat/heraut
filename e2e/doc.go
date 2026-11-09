// Package e2e holds the built-binary tests (ADR-0066): Lane A runs the binary against local
// repositories in every go test run; Lane B (build tag e2e_forge) runs it against the GitHub and
// GitLab sandboxes, on demand only.
package e2e
