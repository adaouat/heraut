package ui

const asciiArt = `
██╗  ██╗███████╗██████╗  █████╗ ██╗   ██╗████████╗
██║  ██║██╔════╝██╔══██╗██╔══██╗██║   ██║╚══██╔══╝
███████║█████╗  ██████╔╝███████║██║   ██║   ██║
██╔══██║██╔══╝  ██╔══██╗██╔══██║██║   ██║   ██║
██║  ██║███████╗██║  ██╗██║  ██║╚██████╔╝   ██║
╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝   ╚═╝`

// CatchPhrase is the tagline displayed under the logo.
const CatchPhrase = "Every release deserves a héraut."

// HelpLong returns the ASCII art + tagline for use as cobra root.Long.
func HelpLong() string {
	return asciiArt + "\n" + CatchPhrase
}

// VersionTemplate returns a cobra text/template string for --version output.
// cobra fills {{.Name}} and {{.Version}} at runtime.
func VersionTemplate() string {
	return asciiArt + "\n\n  " + CatchPhrase + "\n\n  {{.Name}} {{.Version}}\n\n"
}
