package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// --- probe cache, shared by the Files and Info panels ---

type probeEntry struct {
	mod  time.Time
	size int64
	p    *probe
	err  error
	done bool
}

type fileProbedMsg struct {
	key string // absolute path
	p   *probe
	err error
}

// probeSem caps concurrent ffprobe / ncm2mp3 processes.
var probeSem = make(chan struct{}, runtime.NumCPU())

func absPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// relPath shortens an absolute path to be relative to the working directory
// when it lies inside it.
func relPath(p string) string {
	wd, err := os.Getwd()
	if err != nil {
		return p
	}
	if r, err := filepath.Rel(wd, p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}

// requestProbes probes, in the background, every path that is not cached
// yet or has changed on disk since it was probed.
func requestProbes(cache map[string]*probeEntry, paths []string) tea.Cmd {
	var cmds []tea.Cmd
	for _, path := range paths {
		key := absPath(path)
		fi, err := os.Stat(key)
		if err != nil {
			cache[key] = &probeEntry{err: err, done: true}
			continue
		}
		if e, ok := cache[key]; ok && e.mod.Equal(fi.ModTime()) && e.size == fi.Size() {
			continue
		}
		cache[key] = &probeEntry{mod: fi.ModTime(), size: fi.Size()}
		cmds = append(cmds, func() tea.Msg {
			probeSem <- struct{}{}
			defer func() { <-probeSem }()
			p, err := probeFile(key)
			return fileProbedMsg{key: key, p: p, err: err}
		})
	}
	return tea.Batch(cmds...)
}

// --- Files panel ---

var mediaExts = map[string]bool{
	"mp4": true, "mkv": true, "mov": true, "webm": true, "avi": true, "flv": true, "m4v": true,
	"ts": true, "wmv": true, "mpg": true, "mpeg": true, "3gp": true, "gif": true,
	"mp3": true, "m4a": true, "flac": true, "wav": true, "opus": true, "ogg": true, "aac": true,
	"wma": true, "aiff": true, "aif": true, "ape": true, "ncm": true,
}

func isMedia(name string) bool {
	return mediaExts[strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))]
}

type entry struct {
	name  string
	dir   bool
	media bool
	size  int64
}

type filesAction int

const (
	faNone       filesAction = iota
	faDirChanged             // re-probe the new directory
	faInfo                   // show targets() in Info
	faConvert                // send targets() to Convert
)

type files struct {
	dir      string // absolute
	entries  []entry
	err      error
	cursor   int
	offset   int // first visible row
	rows     int // visible rows, set from the window height
	selected map[string]bool
}

func newFiles() files {
	dir, _ := os.Getwd()
	f := files{dir: dir, selected: map[string]bool{}, rows: 10}
	f.load("")
	return f
}

// load re-reads the directory and puts the cursor on focus (or keeps it on
// the current entry): directories first, hidden files skipped.
func (f *files) load(focus string) {
	if focus == "" && f.cursor < len(f.entries) {
		focus = f.entries[f.cursor].name
	}
	des, err := os.ReadDir(f.dir)
	f.err = err
	var dirs, other []entry
	for _, de := range des {
		name := de.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, err := de.Info()
		if err == nil && de.Type()&fs.ModeSymlink != 0 {
			info, err = os.Stat(filepath.Join(f.dir, name))
		}
		if err != nil {
			continue
		}
		if info.IsDir() {
			dirs = append(dirs, entry{name: name, dir: true})
		} else {
			other = append(other, entry{name: name, media: isMedia(name), size: info.Size()})
		}
	}
	f.entries = f.entries[:0]
	if filepath.Dir(f.dir) != f.dir {
		f.entries = append(f.entries, entry{name: "..", dir: true})
	}
	f.entries = append(append(f.entries, dirs...), other...)
	f.cursor = max(0, slices.IndexFunc(f.entries, func(e entry) bool { return e.name == focus }))
	f.scroll()
}

func (f *files) path(e entry) string { return filepath.Join(f.dir, e.name) }

func (f *files) mediaPaths() []string {
	var paths []string
	for _, e := range f.entries {
		if e.media {
			paths = append(paths, f.path(e))
		}
	}
	return paths
}

// targets returns the selected files, or the file under the cursor when
// nothing is selected; paths are made relative to the working directory.
func (f *files) targets() []string {
	var paths []string
	for p := range f.selected {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	if len(paths) == 0 && f.cursor < len(f.entries) && f.entries[f.cursor].media {
		paths = []string{f.path(f.entries[f.cursor])}
	}
	for i, p := range paths {
		paths[i] = relPath(p)
	}
	return paths
}

func (f *files) scroll() {
	f.offset = min(max(f.offset, f.cursor-f.rows+1), f.cursor)
	f.offset = max(0, min(f.offset, len(f.entries)-f.rows))
}

func (f *files) move(d int) {
	f.cursor = min(max(f.cursor+d, 0), max(len(f.entries)-1, 0))
	f.scroll()
}

func (f *files) open(name string) {
	if name == ".." {
		f.up()
		return
	}
	f.dir, f.cursor, f.offset = filepath.Join(f.dir, name), 0, 0
	f.load("")
}

func (f *files) up() {
	prev := filepath.Base(f.dir)
	f.dir, f.offset = filepath.Dir(f.dir), 0
	f.load(prev)
}

func (f *files) handleKey(key string) (bool, filesAction) {
	var cur *entry
	if f.cursor < len(f.entries) {
		cur = &f.entries[f.cursor]
	}
	switch key {
	case "up", "k":
		f.move(-1)
	case "down", "j":
		f.move(1)
	case "pgup":
		f.move(-f.rows)
	case "pgdown":
		f.move(f.rows)
	case "home", "g":
		f.move(-len(f.entries))
	case "end", "G":
		f.move(len(f.entries))
	case "space":
		if cur != nil && cur.media {
			p := f.path(*cur)
			if f.selected[p] {
				delete(f.selected, p)
			} else {
				f.selected[p] = true
			}
		}
		f.move(1)
	case "a": // toggle every media file in this directory
		paths := f.mediaPaths()
		all := len(paths) > 0
		for _, p := range paths {
			all = all && f.selected[p]
		}
		for _, p := range paths {
			if all {
				delete(f.selected, p)
			} else {
				f.selected[p] = true
			}
		}
	case "enter", "right", "l":
		switch {
		case cur == nil:
		case cur.dir:
			f.open(cur.name)
			return true, faDirChanged
		case cur.media && key == "enter":
			return true, faInfo
		}
	case "left", "h", "backspace":
		if filepath.Dir(f.dir) != f.dir {
			f.up()
			return true, faDirChanged
		}
	case "i":
		if len(f.targets()) > 0 {
			return true, faInfo
		}
	case "c":
		if len(f.targets()) > 0 {
			return true, faConvert
		}
	default:
		return false, faNone
	}
	return true, faNone
}

// fit truncates or pads s to exactly w terminal cells.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}

func homeShort(p string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if r, ok := strings.CutPrefix(p, home); ok {
			return "~" + r
		}
	}
	return p
}

func (f *files) view(l lang, cache map[string]*probeEntry, width int) string {
	var b strings.Builder
	avail := width - 4 // body padding
	dir := homeShort(f.dir)
	if over := ansi.StringWidth(dir) - (avail - 16); over > 0 { // keep the tail, it names the folder
		dir = ansi.TruncateLeft(dir, over+1, "…")
	}
	head := sectionStyle.Render(dir)
	if n := len(f.selected); n > 0 {
		head += dimStyle.Render("   " + fmt.Sprintf(l.t("%d selected"), n))
	}
	b.WriteString(head + "\n")
	if f.err != nil {
		return b.String() + errStyle.Render(f.err.Error())
	}

	// Drop the video, then the audio column when the terminal is too narrow.
	const durW, sizeW, minNameW = 10, 10, 16
	videoW, audioW := 22, 20
	free := avail - 4 - durW - sizeW // 4 = cursor + selection mark
	if free-videoW-audioW < minNameW {
		videoW = 0
	}
	if free-videoW-audioW < minNameW {
		audioW = 0
	}
	nameW := max(minNameW, free-videoW-audioW)
	b.WriteString(dimStyle.Render("    "+fit(l.t("Name"), nameW)+fit(l.t("Duration"), durW)+
		fit(l.t("Video"), videoW)+fit(l.t("Audio"), audioW)+l.t("Size")) + "\n")

	if len(f.entries) == 0 {
		b.WriteString("    " + dimStyle.Render(l.t("Empty folder")) + "\n")
	}
	end := min(f.offset+f.rows, len(f.entries))
	for i := f.offset; i < end; i++ {
		e := f.entries[i]
		prefix := "  "
		if i == f.cursor {
			prefix = sectionStyle.Render("▸ ")
		}
		mark := "  "
		if f.selected[f.path(e)] {
			mark = okStyle.Render("✓ ")
		}
		var line string
		switch {
		case e.dir:
			line = sectionStyle.Render(fit(e.name+"/", nameW-1))
		case !e.media:
			line = dimStyle.Render(fit(e.name, nameW+durW+videoW+audioW) + fmtSize(fmtSecs(float64(e.size))))
		default:
			line = fit(e.name, nameW-1) + " " + mediaColumns(cache[f.path(e)], durW, videoW, audioW) + fmtSize(fmtSecs(float64(e.size)))
		}
		b.WriteString(prefix + mark + line + "\n")
	}
	if len(f.entries) > f.rows {
		b.WriteString(dimStyle.Render(fmt.Sprintf("    %d–%d / %d", f.offset+1, end, len(f.entries))) + "\n")
	}
	b.WriteString("\n  " + dimStyle.Render(l.t("space select · a all · enter open · i info · c convert · ← up")))
	return b.String()
}

func mediaColumns(e *probeEntry, durW, videoW, audioW int) string {
	switch {
	case e == nil || !e.done:
		return dimStyle.Render(fit("…", durW+videoW+audioW))
	case e.err != nil:
		return errStyle.Render(fit("✗ "+e.err.Error(), durW+videoW+audioW))
	}
	video, audio := "", ""
	v, a := mainStreams(e.p)
	if v != nil {
		video = fmt.Sprintf("%d×%d %s", v.Width, v.Height, v.CodecName)
	}
	if a != nil {
		parts := []string{a.CodecName}
		if a.Channels > 0 {
			parts = append(parts, fmt.Sprintf("%dch", a.Channels))
		}
		if br := fmtBitRate(a.BitRate); br != "" {
			parts = append(parts, strings.ReplaceAll(br, " ", ""))
		}
		if e.p.NCM != "" {
			parts = append(parts, "(ncm)")
		}
		audio = strings.Join(parts, " ")
	}
	return fit(fmtDuration(e.p.Format.Duration), durW) + fit(video, videoW) + fit(audio, audioW)
}
