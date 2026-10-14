package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// probe mirrors the subset of `ffprobe -print_format json` output we display.
type probe struct {
	Format struct {
		Filename       string            `json:"filename"`
		FormatName     string            `json:"format_name"`
		FormatLongName string            `json:"format_long_name"`
		Duration       string            `json:"duration"`
		Size           string            `json:"size"`
		BitRate        string            `json:"bit_rate"`
		Tags           map[string]string `json:"tags"`
	} `json:"format"`
	Streams []stream `json:"streams"`
}

type stream struct {
	Index         int               `json:"index"`
	CodecType     string            `json:"codec_type"`
	CodecName     string            `json:"codec_name"`
	Profile       string            `json:"profile"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	PixFmt        string            `json:"pix_fmt"`
	AvgFrameRate  string            `json:"avg_frame_rate"`
	SampleRate    string            `json:"sample_rate"`
	Channels      int               `json:"channels"`
	ChannelLayout string            `json:"channel_layout"`
	BitRate       string            `json:"bit_rate"`
	Tags          map[string]string `json:"tags"`
	Disposition   map[string]int    `json:"disposition"`
}

type probeMsg struct {
	probe *probe
	err   error
}

func runProbe(path string) tea.Cmd {
	return func() tea.Msg {
		p, err := probeFile(path)
		return probeMsg{probe: p, err: err}
	}
}

func probeFile(path string) (*probe, error) {
	out, err := exec.Command("ffprobe", "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", safeInput(path)).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			err = errors.New(strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	var p probe
	if err := json.Unmarshal(out, &p); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}
	return &p, nil
}

// safeInput keeps a leading "-" from being parsed as an ffmpeg/ffprobe option.
func safeInput(path string) string {
	if strings.HasPrefix(path, "-") {
		return "./" + path
	}
	return path
}

var (
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	labelStyle   = lipgloss.NewStyle().Width(13).Foreground(lipgloss.Color("8"))
	errStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

func renderProbe(p *probe, l lang) string {
	var b strings.Builder
	kv := func(label, value string) {
		if value != "" {
			b.WriteString("  " + labelStyle.Render(l.t(label)) + value + "\n")
		}
	}

	f := p.Format
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(filepath.Base(f.Filename)) + "\n\n")
	b.WriteString(sectionStyle.Render(l.t("General")) + "\n")
	format := f.FormatLongName
	if f.FormatName != "" {
		format += " (" + f.FormatName + ")"
	}
	kv("Format", format)
	kv("Duration", fmtDuration(f.Duration))
	kv("Size", fmtSize(f.Size))
	kv("Bitrate", fmtBitRate(f.BitRate))
	kv("Title", f.Tags["title"])

	for _, s := range p.Streams {
		b.WriteString("\n" + sectionStyle.Render(fmt.Sprintf("#%d %s", s.Index, l.t(titleCase(s.CodecType)))) + "\n")
		codec := s.CodecName
		if s.Profile != "" {
			codec += " (" + s.Profile + ")"
		}
		kv("Codec", codec)
		if s.Width > 0 {
			kv("Resolution", fmt.Sprintf("%d×%d", s.Width, s.Height))
		}
		kv("Frame rate", fmtFrameRate(s.AvgFrameRate))
		kv("Pixel format", s.PixFmt)
		kv("Sample rate", fmtSampleRate(s.SampleRate))
		if s.Channels > 0 {
			ch := strconv.Itoa(s.Channels)
			if s.ChannelLayout != "" {
				ch += " (" + s.ChannelLayout + ")"
			}
			kv("Channels", ch)
		}
		kv("Bitrate", fmtBitRate(s.BitRate))
		if language := s.Tags["language"]; language != "und" {
			kv("Language", language)
		}
		kv("Title", s.Tags["title"])
	}
	return strings.TrimRight(b.String(), "\n")
}

func titleCase(s string) string {
	if s == "" {
		return "Unknown"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func fmtDuration(s string) string {
	sec, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return ""
	}
	t := int(sec)
	return fmt.Sprintf("%02d:%02d:%02d", t/3600, t/60%60, t%60)
}

func fmtSize(s string) string {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return ""
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	i := 0
	for n >= 1024 && i < len(units)-1 {
		n /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", n, units[i])
}

func fmtBitRate(s string) string {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return ""
	}
	if n >= 1e6 {
		return fmt.Sprintf("%.2f Mb/s", n/1e6)
	}
	return fmt.Sprintf("%.0f kb/s", n/1e3)
}

func fmtSampleRate(s string) string {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatFloat(n/1000, 'f', -1, 64) + " kHz"
}

// fmtFrameRate turns ffprobe's rational "30000/1001" into "29.97 fps".
func fmtFrameRate(s string) string {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		return ""
	}
	n, err1 := strconv.ParseFloat(num, 64)
	d, err2 := strconv.ParseFloat(den, 64)
	if err1 != nil || err2 != nil || n == 0 || d == 0 {
		return ""
	}
	return strconv.FormatFloat(math.Round(n/d*100)/100, 'f', -1, 64) + " fps"
}
