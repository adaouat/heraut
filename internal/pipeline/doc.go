// Package pipeline runs the release and changelog flows step by step: resolve, changelog, commit,
// tag, push, notes and publish, with hooks and dry runs. It works on interfaces and never builds
// its own dependencies.
package pipeline
