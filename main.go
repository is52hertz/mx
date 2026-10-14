// mx is a TUI front-end that unifies media info (ffprobe), download (yt-dlp)
// and conversion (ffmpeg) behind one command.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

const usage = `mx — media info / download / convert

Usage:
  mx                  open the TUI
  mx info <file>      open the Info panel
  mx dl <url>         open the Download panel
  mx cv <file>... [fmt]  open the Convert panel (batch)
`

func main() {
	tab, args, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprint(os.Stderr, err, "\n\n", usage)
		os.Exit(2)
	}
	if _, err := tea.NewProgram(newModel(tab, args)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mx:", err)
		os.Exit(1)
	}
}

func parseArgs(argv []string) (tab, []string, error) {
	if len(argv) == 0 {
		return tabInfo, nil, nil
	}
	switch argv[0] {
	case "info", "i":
		return tabInfo, argv[1:], nil
	case "dl", "d":
		return tabDownload, argv[1:], nil
	case "cv", "c":
		return tabConvert, argv[1:], nil
	case "-h", "--help", "help":
		fmt.Print(usage)
		os.Exit(0)
	}
	return 0, nil, fmt.Errorf("unknown command %q", argv[0])
}
