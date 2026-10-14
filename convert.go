package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type cvRow int

const (
	rowFormat cvRow = iota
	rowQuality
	rowScale
	rowTrim
	rowCount
)

type cvState int

const (
	cvIdle cvState = iota
	cvRunning
	cvDone
)

var scaleHeights = []int{0, 1080, 720, 480}

type cvJob struct {
	in, out string
	probe   *probe
	err     error
	done    bool
	elapsed time.Duration
}

type cvProbedMsg struct {
	probes []*probe
	err    error
}

// convert is the Convert panel: a form of option rows, then a batch runner.
type convert struct {
	files     []string
	presetExt string // format given on the command line, e.g. `mx cv a.mov mp3`
	probes    []*probe
	probeErr  error

	row      cvRow
	fmtIdx   int // -1 when no format fits the inputs
	q        quality
	scaleIdx int
	trim     trimMode
	secs     string // seconds as typed, e.g. "90" or "1:30"

	state    cvState
	jobs     []cvJob
	cur      int
	run      *runner
	outSecs  float64
	speed    float64
	started  time.Time
	canceled bool
}

func newConvert(args []string) convert {
	c := convert{files: args, fmtIdx: -1, q: qMedium}
	if n := len(args); n > 1 {
		last := strings.ToLower(strings.TrimPrefix(args[n-1], "."))
		if _, err := os.Stat(args[n-1]); err != nil && formatIndex(last) >= 0 {
			c.files, c.presetExt = args[:n-1], last
		}
	}
	return c
}

func formatIndex(ext string) int {
	for i, f := range formats {
		if f.ext == ext {
			return i
		}
	}
	return -1
}

func (c *convert) init() tea.Cmd {
	if len(c.files) == 0 {
		return nil
	}
	files := c.files
	return func() tea.Msg {
		probes := make([]*probe, len(files))
		for i, f := range files {
			p, err := probeFile(f)
			if err != nil {
				return cvProbedMsg{err: fmt.Errorf("%s: %w", f, err)}
			}
			probes[i] = p
		}
		return cvProbedMsg{probes: probes}
	}
}

// --- option availability ---

func (c *convert) formatUsable(i int) bool {
	for _, p := range c.probes {
		if !formats[i].usable(p) {
			return false
		}
	}
	return len(c.probes) > 0
}

func (c *convert) copyOK() bool {
	if c.fmtIdx < 0 || (formats[c.fmtIdx].video && c.scaleIdx != 0) {
		return false
	}
	for _, p := range c.probes {
		if !formats[c.fmtIdx].canCopy(p) {
			return false
		}
	}
	return true
}

func (c *convert) qualityOK(q quality) bool {
	f := formats[c.fmtIdx]
	switch {
	case q == qCopy:
		return c.copyOK()
	case f.lossless:
		return q == qMedium
	}
	return true
}

func (c *convert) rowVisible(r cvRow) bool {
	return r != rowScale || formats[c.fmtIdx].video
}

func (c *convert) trimSecs() (float64, bool) {
	if c.trim == trimNone {
		return 0, true
	}
	return parseSecs(c.secs)
}

func (c *convert) opts() cvOpts {
	secs, _ := c.trimSecs()
	return cvOpts{f: formats[c.fmtIdx], q: c.q, height: scaleHeights[c.scaleIdx], trim: c.trim, secs: secs}
}

// selectFormat switches format and defaults quality to stream copy when possible.
func (c *convert) selectFormat(i int) {
	c.fmtIdx = i
	if !formats[i].video {
		c.scaleIdx = 0
	}
	c.q = qMedium
	if c.copyOK() {
		c.q = qCopy
	}
}

// --- update ---

func (c *convert) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case cvProbedMsg:
		c.probes, c.probeErr = msg.probes, msg.err
		if msg.err != nil {
			return nil
		}
		i := formatIndex(c.presetExt)
		if i < 0 || !c.formatUsable(i) {
			i = -1
			for j := range formats {
				if c.formatUsable(j) {
					i = j
					break
				}
			}
		}
		if i >= 0 {
			c.selectFormat(i)
		}
	case cvProgressMsg:
		if msg.run == c.run {
			c.outSecs, c.speed = msg.outSecs, msg.speed
			return c.run.listen()
		}
	case cvJobDoneMsg:
		if msg.run == c.run {
			j := &c.jobs[c.cur]
			j.err, j.done, j.elapsed = msg.err, true, time.Since(c.started)
			c.cur++
			return c.startNext()
		}
	}
	return nil
}

// handleKey returns handled=false for keys the global handler should see.
func (c *convert) handleKey(key string) (bool, tea.Cmd) {
	switch c.state {
	case cvRunning:
		if key == "esc" {
			c.cancel()
			return true, nil
		}
		return false, nil
	case cvDone:
		if key == "enter" {
			c.state = cvIdle
			return true, nil
		}
		return false, nil
	}
	if c.fmtIdx < 0 {
		return false, nil
	}

	if c.row == rowTrim && c.trim != trimNone {
		switch {
		case len(key) == 1 && strings.ContainsAny(key, "0123456789:."):
			c.secs += key
			return true, nil
		case key == "backspace" && c.secs != "":
			c.secs = c.secs[:len(c.secs)-1]
			return true, nil
		}
	}

	switch key {
	case "up", "k":
		for r := c.row - 1; r >= 0; r-- {
			if c.rowVisible(r) {
				c.row = r
				break
			}
		}
	case "down", "j":
		for r := c.row + 1; r < rowCount; r++ {
			if c.rowVisible(r) {
				c.row = r
				break
			}
		}
	case "left", "h":
		c.step(-1)
	case "right", "l":
		c.step(1)
	case "enter":
		if _, ok := c.trimSecs(); ok {
			return true, c.start()
		}
	default:
		return false, nil
	}
	return true, nil
}

// step moves the focused row's value by d, skipping unavailable options.
func (c *convert) step(d int) {
	switch c.row {
	case rowFormat:
		for i := c.fmtIdx + d; i >= 0 && i < len(formats); i += d {
			if c.formatUsable(i) {
				c.selectFormat(i)
				return
			}
		}
	case rowQuality:
		for q := int(c.q) + d; q >= int(qCopy) && q <= int(qSmall); q += d {
			if c.qualityOK(quality(q)) {
				c.q = quality(q)
				return
			}
		}
	case rowScale:
		c.scaleIdx = min(max(c.scaleIdx+d, 0), len(scaleHeights)-1)
		if c.q == qCopy && !c.copyOK() {
			c.q = qMedium
		}
	case rowTrim:
		c.trim = trimMode(min(max(int(c.trim)+d, int(trimNone)), int(trimTail)))
	}
}

// --- running ---

func (c *convert) start() tea.Cmd {
	c.jobs = make([]cvJob, len(c.files))
	for i, f := range c.files {
		c.jobs[i] = cvJob{in: f, probe: c.probes[i]}
	}
	c.state, c.cur, c.canceled = cvRunning, 0, false
	return c.startNext()
}

func (c *convert) startNext() tea.Cmd {
	for ; c.cur < len(c.jobs); c.cur++ {
		j := &c.jobs[c.cur]
		out, err := outputName(j.in, formats[c.fmtIdx].ext, true)
		if err != nil {
			j.err, j.done = err, true
			continue
		}
		j.out = out
		c.outSecs, c.speed, c.started = 0, 0, time.Now()
		c.run = startFFmpeg(ffmpegArgs(j.in, out, c.opts(), j.probe), out)
		return c.run.listen()
	}
	c.state, c.run = cvDone, nil
	return nil
}

func (c *convert) cancel() {
	if c.run != nil {
		c.run.stop()
		c.jobs[c.cur].err = errCanceled
		c.jobs[c.cur].done = true
	}
	c.state, c.run, c.canceled = cvDone, nil, true
}

var errCanceled = fmt.Errorf("canceled")

// expected returns the output duration of a job, for progress percentages.
func (c *convert) expected(p *probe) float64 {
	d, _ := strconv.ParseFloat(p.Format.Duration, 64)
	if secs, ok := c.trimSecs(); ok && c.trim != trimNone {
		d = min(d, secs)
	}
	return d
}

// --- view ---

var (
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selectedStyle = lipgloss.NewStyle().Reverse(true)
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
)

func (c *convert) view(l lang) string {
	switch {
	case len(c.files) == 0:
		return l.t("Usage: mx cv <file>... [format]")
	case c.probeErr != nil:
		return errStyle.Render("ffprobe: " + c.probeErr.Error())
	case c.probes == nil:
		return fmt.Sprintf(l.t("Probing %d file(s) …"), len(c.files))
	case c.fmtIdx < 0:
		return c.inputView(l) + "\n\n" + errStyle.Render(l.t("No target format fits these inputs"))
	case c.state != cvIdle:
		return c.runView(l)
	}

	var b strings.Builder
	b.WriteString(c.inputView(l) + "\n\n")

	row := func(r cvRow, label string, values []string, selected int, ok func(int) bool, suffix string) {
		if !c.rowVisible(r) {
			return
		}
		prefix := "  "
		lbl := labelStyle.Render(l.t(label))
		if r == c.row {
			prefix = sectionStyle.Render("▸ ")
			lbl = labelStyle.Bold(true).Render(l.t(label))
		}
		parts := make([]string, len(values))
		for i, v := range values {
			v = " " + l.t(v) + " "
			switch {
			case i == selected:
				parts[i] = selectedStyle.Render(v)
			case !ok(i):
				parts[i] = dimStyle.Strikethrough(true).Render(v)
			default:
				parts[i] = v
			}
		}
		if r == rowFormat { // separate video and audio formats
			for i, f := range formats {
				if !f.video {
					parts[i] = dimStyle.Render("│ ") + parts[i]
					break
				}
			}
		}
		b.WriteString(prefix + lbl + strings.Join(parts, " ") + suffix + "\n")
	}

	exts := make([]string, len(formats))
	for i, f := range formats {
		exts[i] = f.ext
	}
	row(rowFormat, "Format", exts, c.fmtIdx, c.formatUsable, "   "+dimStyle.Render(l.t(formats[c.fmtIdx].desc)))

	qLabels := []string{"⚡Copy", "High", "Medium", "Small"}
	if formats[c.fmtIdx].lossless {
		qLabels[qMedium] = "Lossless"
	}
	row(rowQuality, "Quality", qLabels, int(c.q), func(i int) bool { return c.qualityOK(quality(i)) }, "")

	row(rowScale, "Resolution", []string{"Original", "1080p", "720p", "480p"}, c.scaleIdx, func(int) bool { return true }, "")

	secs := ""
	if c.trim != trimNone {
		secs = "   " + c.secs
		if c.row == rowTrim {
			secs += "▏"
		}
		secs += "s"
	}
	row(rowTrim, "Trim", []string{"All", "First", "Last"}, int(c.trim), func(int) bool { return true }, secs)

	b.WriteString("\n")
	outs := make([]string, 0, 2)
	for i, f := range c.files {
		if i == 1 && len(c.files) > 2 {
			outs = append(outs, "…")
			continue
		}
		if i > 0 && i < len(c.files)-1 {
			continue
		}
		out, _ := outputName(f, formats[c.fmtIdx].ext, false)
		outs = append(outs, filepath.Base(out))
	}
	b.WriteString("  " + labelStyle.Render(l.t("Output")) + " " + strings.Join(outs, " ") + dimStyle.Render(" "+l.t("(same folder)")) + "\n")
	out, _ := outputName(c.files[0], formats[c.fmtIdx].ext, false)
	b.WriteString("  " + labelStyle.Render(l.t("Command")) + " " + dimStyle.Render(shellJoin(append([]string{"ffmpeg"}, ffmpegArgs(c.files[0], out, c.opts(), c.probes[0])...))) + "\n\n")

	if hint := c.hint(l); hint != "" {
		b.WriteString("  " + hint + "\n")
	}
	keys := l.t("↑↓ option · ←→ change · enter start")
	if c.row == rowTrim && c.trim != trimNone {
		keys += l.t(" · 0-9 seconds")
	}
	b.WriteString("  " + dimStyle.Render(keys))
	return b.String()
}

func (c *convert) hint(l lang) string {
	if _, ok := c.trimSecs(); !ok {
		return sectionStyle.Render(l.t("Type seconds, e.g. 30 or 1:30"))
	}
	if c.q == qCopy {
		h := l.t("⚡ Copy streams: no re-encode, fast and lossless")
		if c.trim != trimNone {
			h += l.t(" · cuts at keyframes, may be off by a few seconds")
		}
		return okStyle.Render(h)
	}
	if c.row == rowQuality {
		if formats[c.fmtIdx].video && c.scaleIdx != 0 {
			return dimStyle.Render(l.t("⚡ needs original resolution"))
		}
		if !c.copyOK() {
			return dimStyle.Render(l.t("⚡ unavailable: codecs don't fit this format"))
		}
	}
	return ""
}

func (c *convert) inputView(l lang) string {
	lbl := labelStyle.Render(l.t("Input")) + " "
	if len(c.probes) == 1 {
		p := c.probes[0]
		parts := []string{filepath.Base(c.files[0]), fmtDuration(p.Format.Duration)}
		v, a := mainStreams(p)
		if v != nil {
			parts = append(parts, fmt.Sprintf("%d×%d %s", v.Width, v.Height, v.CodecName))
		}
		if a != nil {
			parts = append(parts, fmt.Sprintf("%s %dch", a.CodecName, a.Channels))
		}
		parts = append(parts, fmtSize(p.Format.Size))
		return "  " + lbl + strings.Join(parts, " · ")
	}

	var dur, size float64
	for _, p := range c.probes {
		d, _ := strconv.ParseFloat(p.Format.Duration, 64)
		s, _ := strconv.ParseFloat(p.Format.Size, 64)
		dur, size = dur+d, size+s
	}
	names := make([]string, 0, 4)
	for i, f := range c.files {
		if i == 3 {
			names = append(names, fmt.Sprintf(l.t("+%d more"), len(c.files)-3))
			break
		}
		names = append(names, filepath.Base(f))
	}
	return "  " + lbl + fmt.Sprintf(l.t("%d files"), len(c.files)) + " · " +
		fmtDuration(fmtSecs(dur)) + " · " + fmtSize(fmtSecs(size)) + "\n" +
		"  " + labelStyle.Render("") + " " + dimStyle.Render(strings.Join(names, "  "))
}

func (c *convert) runView(l lang) string {
	var b strings.Builder
	ok, failed := 0, 0
	for i, j := range c.jobs {
		name := filepath.Base(j.in)
		if j.out != "" {
			name += " → " + filepath.Base(j.out)
		}
		switch {
		case j.done && j.err == nil:
			ok++
			b.WriteString("  " + okStyle.Render("✓ ") + name + dimStyle.Render("  "+fmtClock(j.elapsed.Seconds())) + "\n")
		case j.done && j.err == errCanceled:
			b.WriteString("  " + dimStyle.Render("– "+name+"  "+l.t("canceled")) + "\n")
		case j.done:
			failed++
			b.WriteString("  " + errStyle.Render("✗ ") + name + "\n    " + errStyle.Render(j.err.Error()) + "\n")
		case c.state == cvRunning && i == c.cur:
			b.WriteString("  " + sectionStyle.Render("▸ ") + name + "  " + c.progressView(l) + "\n")
		case c.canceled:
			b.WriteString("  " + dimStyle.Render("– "+name+"  "+l.t("skipped")) + "\n")
		default:
			b.WriteString("    " + dimStyle.Render(name) + "\n")
		}
	}
	b.WriteString("\n")
	if c.state == cvRunning {
		b.WriteString(fmt.Sprintf("  %d/%d · ", c.cur, len(c.jobs)) + dimStyle.Render(l.t("esc cancel (unfinished file is deleted)")))
		return b.String()
	}
	summary := "Done: %d succeeded, %d failed"
	if c.canceled {
		summary = "Canceled: %d succeeded, %d failed"
	}
	b.WriteString("  " + fmt.Sprintf(l.t(summary), ok, failed) + "\n  " + dimStyle.Render(l.t("enter back · q quit")))
	return b.String()
}

func (c *convert) progressView(l lang) string {
	total := c.expected(c.jobs[c.cur].probe)
	frac := 0.0
	if total > 0 {
		frac = min(c.outSecs/total, 1)
	}
	const width = 24
	filled := int(frac * width)
	s := sectionStyle.Render(strings.Repeat("█", filled)) + dimStyle.Render(strings.Repeat("░", width-filled)) +
		fmt.Sprintf(" %3.0f%%", frac*100)
	if c.speed > 0 && total > 0 {
		s += dimStyle.Render(fmt.Sprintf("  "+l.t("left %s")+" · %.1fx", fmtClock((total-c.outSecs)/c.speed), c.speed))
	}
	return s
}

func fmtClock(sec float64) string {
	t := int(max(sec, 0))
	if t >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", t/3600, t/60%60, t%60)
	}
	return fmt.Sprintf("%02d:%02d", t/60, t%60)
}

// shellJoin renders argv as a copy-pasteable shell command for the preview.
func shellJoin(argv []string) string {
	out := make([]string, len(argv))
	for i, a := range argv {
		if a == "" || strings.ContainsAny(a, " '\"\\$`!*?[]();&|<>{}~#") {
			a = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
		out[i] = a
	}
	return strings.Join(out, " ")
}
