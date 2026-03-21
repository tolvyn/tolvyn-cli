package main

import (
	"io/fs"
	"os"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

// isTTY returns true when stdout is an interactive terminal.
func isTTY() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & fs.ModeCharDevice) != 0
}

var useColor = true // set false by --no-color or when piped

func green(s string) string {
	if !useColor {
		return s
	}
	return colorGreen + s + colorReset
}

func red(s string) string {
	if !useColor {
		return s
	}
	return colorRed + s + colorReset
}

func yellow(s string) string {
	if !useColor {
		return s
	}
	return colorYellow + s + colorReset
}

func cyan(s string) string {
	if !useColor {
		return s
	}
	return colorCyan + s + colorReset
}
