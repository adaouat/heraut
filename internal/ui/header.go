package ui

import (
	"os"

	"charm.land/lipgloss/v2"

	forgeui "github.com/adaouat/forge/ui"
)

const asciiArt = `
██╗  ██╗███████╗██████╗  █████╗ ██╗   ██╗████████╗
██║  ██║██╔════╝██╔══██╗██╔══██╗██║   ██║╚══██╔══╝
███████║█████╗  ██████╔╝███████║██║   ██║   ██║
██╔══██║██╔══╝  ██╔══██╗██╔══██║██║   ██║   ██║
██║  ██║███████╗██║  ██╗██║  ██║╚██████╔╝   ██║
╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝    ╚═╝`

// CatchPhrase is the tagline displayed under the logo.
const CatchPhrase = "Every release deserves a héraut."

// goldStyle renders the banner in heraut's gold accent, adapting to the terminal's
// light/dark background the same way fang resolves its color scheme.
func goldStyle() lipgloss.Style {
	a := Accent()
	ld := lipgloss.LightDark(lipgloss.HasDarkBackground(os.Stdin, os.Stdout))
	return lipgloss.NewStyle().Foreground(ld(a.Light, a.Dark)).Bold(true)
}

// HelpLong returns the ASCII art + tagline, gold-accented, for use as cobra root.Long.
func HelpLong() string {
	s := goldStyle()
	return forgeui.HelpLong(s.Render(asciiArt), s.Render(CatchPhrase))
}

// VersionTemplate returns a cobra text/template string for --version output.
// cobra fills {{.Name}} and {{.Version}} at runtime.
func VersionTemplate() string {
	s := goldStyle()
	return forgeui.VersionTemplate(s.Render(asciiArt), s.Render(CatchPhrase))
}
