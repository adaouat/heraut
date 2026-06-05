package ui

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	forgeui "github.com/adaouat/forge/ui"
)

// Accent is heraut's brand — Heraldic: gold title/program/flags, azure commands — over forge's
// shared palette (forge ADR-0010).
func Accent() forgeui.Accent {
	return forgeui.Accent{
		Light:          lipgloss.Color("#9E6A03"),
		Dark:           lipgloss.Color("#E3B341"),
		SecondaryLight: lipgloss.Color("#0969DA"),
		SecondaryDark:  lipgloss.Color("#79C0FF"),
	}
}

// HuhTheme is heraut's branded interactive-form theme.
func HuhTheme() huh.ThemeFunc { return forgeui.HuhTheme(Accent()) }
