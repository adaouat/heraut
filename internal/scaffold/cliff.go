package scaffold

// IsCliffGenerator reports whether gen is the git-cliff generator identifier.
func IsCliffGenerator(gen string) bool {
	return gen == "git-cliff"
}
