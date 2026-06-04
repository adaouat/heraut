package main

import (
	"image/color"

	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"

	"github.com/adaouat/forge/ui"
)

// colorScheme is heraut's fang theme: the Heraldic accent (gold title/program/flags, azure
// commands) over forge's shared structural palette. See forge ADR-0008.
func colorScheme(c lipgloss.LightDarkFunc) fang.ColorScheme {
	p := ui.NewPalette(c)
	accent := c(lipgloss.Color("#9E6A03"), lipgloss.Color("#E3B341"))    // gold
	secondary := c(lipgloss.Color("#0969DA"), lipgloss.Color("#79C0FF")) // azure
	return fang.ColorScheme{
		Base:           p.Text,
		Title:          accent,
		Description:    p.Muted,
		Codeblock:      p.Muted,
		Program:        accent,
		DimmedArgument: p.Dim,
		Comment:        p.Muted,
		Flag:           accent,
		FlagDefault:    p.Dim,
		Command:        secondary,
		QuotedString:   p.Success,
		Argument:       p.Argument,
		Help:           p.Muted,
		Dash:           p.Muted,
		ErrorHeader:    [2]color.Color{lipgloss.Color("#FFFFFF"), p.Error},
		ErrorDetails:   p.Error,
	}
}
