package output

import (
	"os"

	"golang.org/x/term"
)

// ANSI color codes.
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
)

// ColorWriter controls whether ANSI color codes are emitted.
type ColorWriter struct {
	Enabled bool
}

// NewColorWriter returns a ColorWriter that auto-detects terminal support.
// If forceOn is true, color is enabled regardless of terminal detection.
// If forceOff is true, color is disabled. forceOff takes precedence.
func NewColorWriter(forceOn, forceOff bool) *ColorWriter {
	enabled := term.IsTerminal(int(os.Stdout.Fd()))
	if forceOn {
		enabled = true
	}
	if forceOff {
		enabled = false
	}
	return &ColorWriter{Enabled: enabled}
}

func (c *ColorWriter) Bold(s string) string {
	if !c.Enabled {
		return s
	}
	return bold + s + reset
}

func (c *ColorWriter) Dim(s string) string {
	if !c.Enabled {
		return s
	}
	return dim + s + reset
}

func (c *ColorWriter) Green(s string) string {
	if !c.Enabled {
		return s
	}
	return green + s + reset
}

func (c *ColorWriter) Red(s string) string {
	if !c.Enabled {
		return s
	}
	return red + s + reset
}

func (c *ColorWriter) Yellow(s string) string {
	if !c.Enabled {
		return s
	}
	return yellow + s + reset
}

func (c *ColorWriter) Cyan(s string) string {
	if !c.Enabled {
		return s
	}
	return cyan + s + reset
}
