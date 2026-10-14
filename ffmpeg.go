package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type quality int

const (
	qCopy quality = iota // stream copy, no re-encode
	qHigh
	qMedium
	qSmall
)

type trimMode int

const (
	trimNone trimMode = iota
	trimHead          // keep the first N seconds
	trimTail          // keep the last N seconds
)

type format struct {
	ext      string
	video    bool
	lossless bool
	desc     string
	anyCopy  bool     // container accepts any codec for stream copy (mkv)
	vcopy    []string // video codecs that can be stream-copied into this container
	acopy    []string // audio codecs that can be stream-copied into this container
}

var formats = []format{
	{ext: "mp4", video: true, desc: "H.264 + AAC", vcopy: []string{"h264", "hevc", "av1", "mpeg4"}, acopy: []string{"aac", "mp3", "ac3", "eac3", "alac", "opus"}},
	{ext: "mkv", video: true, desc: "H.264 + AAC", anyCopy: true},
	{ext: "webm", video: true, desc: "VP9 + Opus", vcopy: []string{"vp8", "vp9", "av1"}, acopy: []string{"opus", "vorbis"}},
	{ext: "gif", video: true, desc: "animated, no audio"},
	{ext: "mp3", desc: "MP3", acopy: []string{"mp3"}},
	{ext: "m4a", desc: "AAC", acopy: []string{"aac", "alac"}},
	{ext: "flac", lossless: true, desc: "FLAC", acopy: []string{"flac"}},
	{ext: "wav", lossless: true, desc: "PCM 16-bit", acopy: []string{"pcm_s16le"}},
	{ext: "opus", desc: "Opus", acopy: []string{"opus"}},
}

// mainStreams returns the first real video stream (cover art excluded) and
// the first audio stream; either may be nil.
func mainStreams(p *probe) (v, a *stream) {
	for i := range p.Streams {
		s := &p.Streams[i]
		switch {
		case s.CodecType == "video" && v == nil && s.Disposition["attached_pic"] == 0:
			v = s
		case s.CodecType == "audio" && a == nil:
			a = s
		}
	}
	return v, a
}

func (f format) usable(p *probe) bool {
	v, a := mainStreams(p)
	if f.video {
		return v != nil
	}
	return a != nil
}

func (f format) canCopy(p *probe) bool {
	if f.ext == "gif" || !f.usable(p) {
		return false
	}
	if f.anyCopy {
		return true
	}
	v, a := mainStreams(p)
	if f.video {
		return slices.Contains(f.vcopy, v.CodecName) && (a == nil || slices.Contains(f.acopy, a.CodecName))
	}
	return slices.Contains(f.acopy, a.CodecName)
}

type cvOpts struct {
	f      format
	q      quality
	height int // 0 keeps the original resolution
	trim   trimMode
	secs   float64
}

// ffmpegArgs builds the conversion arguments (without the runtime flags that
// runFFmpeg adds), so they can double as the command preview.
func ffmpegArgs(in, out string, o cvOpts, p *probe) []string {
	var args []string
	if o.trim == trimTail {
		args = append(args, "-sseof", fmtSecs(-o.secs))
	}
	args = append(args, "-i", safeInput(in))
	if o.trim == trimHead {
		args = append(args, "-t", fmtSecs(o.secs))
	}

	f := o.f
	if o.q == qCopy {
		switch {
		case f.anyCopy:
			args = append(args, "-map", "0", "-map", "-0:d", "-c", "copy")
		case f.video:
			args = append(args, "-c", "copy", "-sn", "-dn")
			if v, _ := mainStreams(p); f.ext == "mp4" && v.CodecName == "hevc" {
				args = append(args, "-tag:v", "hvc1") // playable in QuickTime
			}
		default:
			args = append(args, "-vn", "-c:a", "copy")
		}
		return append(args, safeInput(out))
	}

	pick := func(high, medium, small string) string {
		return [...]string{qHigh: high, qMedium: medium, qSmall: small}[o.q]
	}
	scale := ""
	if o.height > 0 {
		scale = fmt.Sprintf("scale=-2:'trunc(min(ih,%d)/2)*2'", o.height)
	}
	withScale := func(args []string) []string {
		if scale != "" {
			return append(args, "-vf", scale)
		}
		return args
	}

	switch f.ext {
	case "mp4", "mkv":
		args = withScale(args)
		args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", pick("18", "23", "28"),
			"-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", pick("256k", "160k", "96k"))
		if f.ext == "mp4" {
			args = append(args, "-sn", "-dn", "-movflags", "+faststart")
		} else {
			args = append(args, "-c:s", "copy")
		}
	case "webm":
		args = withScale(args)
		args = append(args, "-c:v", "libvpx-vp9", "-crf", pick("24", "32", "40"), "-b:v", "0", "-row-mt", "1",
			"-c:a", "libopus", "-b:a", pick("192k", "128k", "64k"), "-sn", "-dn")
	case "gif":
		vf := "fps=" + pick("20", "15", "10")
		if scale != "" {
			vf += "," + scale
		}
		args = append(args, "-vf", vf+",split[a][b];[a]palettegen[p];[b][p]paletteuse", "-an")
	case "mp3":
		args = append(args, "-vn", "-c:a", "libmp3lame", "-q:a", pick("0", "2", "5"))
	case "m4a":
		args = append(args, "-vn", "-c:a", "aac", "-b:a", pick("256k", "160k", "96k"))
	case "flac":
		args = append(args, "-vn", "-c:a", "flac")
	case "wav":
		args = append(args, "-vn", "-c:a", "pcm_s16le")
	case "opus":
		args = append(args, "-vn", "-c:a", "libopus", "-b:a", pick("160k", "96k", "48k"))
	}
	return append(args, safeInput(out))
}

func fmtSecs(s float64) string { return strconv.FormatFloat(s, 'f', -1, 64) }

// parseSecs accepts "90", "1:30" or "1:02:03".
func parseSecs(s string) (float64, bool) {
	var total float64
	for part := range strings.SplitSeq(s, ":") {
		n, err := strconv.ParseFloat(part, 64)
		if err != nil || n < 0 {
			return 0, false
		}
		total = total*60 + n
	}
	return total, total > 0
}

// outputName returns the first free "<base>.<ext>" / "<base> (N).<ext>" next
// to in. With reserve, it atomically creates the file (O_EXCL) so the name
// cannot be taken, and so a failed run only ever deletes a file mx created.
func outputName(in, ext string, reserve bool) (string, error) {
	dir := filepath.Dir(in)
	base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
	for i := 0; ; i++ {
		name := base + "." + ext
		if i > 0 {
			name = fmt.Sprintf("%s (%d).%s", base, i, ext)
		}
		path := filepath.Join(dir, name)
		if !reserve {
			if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
				return path, nil
			}
			continue
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return path, f.Close()
	}
}

type cvProgressMsg struct {
	run     *runner
	outSecs float64
	speed   float64
}

type cvJobDoneMsg struct {
	run *runner
	err error
}

// runner owns one running ffmpeg process.
type runner struct {
	cancel context.CancelFunc
	msgs   chan tea.Msg
	done   chan struct{}
}

// startFFmpeg runs ffmpeg in the background; out must already be reserved
// by outputName and is deleted if the run fails or is canceled.
func startFFmpeg(args []string, out string) *runner {
	ctx, cancel := context.WithCancel(context.Background())
	r := &runner{cancel: cancel, msgs: make(chan tea.Msg), done: make(chan struct{})}
	go func() {
		defer close(r.msgs)
		err := r.exec(ctx, args)
		if err != nil {
			os.Remove(out)
		}
		close(r.done)
		select {
		case r.msgs <- cvJobDoneMsg{run: r, err: err}:
		case <-ctx.Done():
		}
	}()
	return r
}

func (r *runner) exec(ctx context.Context, args []string) error {
	// -y only overwrites the empty placeholder reserved by outputName.
	full := append([]string{"-hide_banner", "-nostdin", "-y", "-loglevel", "error", "-progress", "pipe:1", "-nostats"}, args...)
	cmd := exec.CommandContext(ctx, "ffmpeg", full...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	var outSecs float64
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), "=")
		switch k {
		case "out_time_us":
			if n, err := strconv.ParseFloat(v, 64); err == nil {
				outSecs = n / 1e6
			}
		case "speed":
			speed, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(v), "x"), 64)
			select {
			case r.msgs <- cvProgressMsg{run: r, outSecs: outSecs, speed: speed}:
			case <-ctx.Done():
			}
		}
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if lines := strings.Split(strings.TrimSpace(stderr.String()), "\n"); lines[len(lines)-1] != "" {
			return errors.New(lines[len(lines)-1])
		}
		return err
	}
	return nil
}

// stop kills ffmpeg and waits until its partial output is removed.
func (r *runner) stop() {
	r.cancel()
	<-r.done
}

func (r *runner) listen() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-r.msgs
		if !ok {
			return nil
		}
		return msg
	}
}
