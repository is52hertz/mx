package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ncm2mp3 (https://github.com/Johnserf-Seed/ncm2mp3) decrypts NetEase Cloud
// Music .ncm files to the audio format stored inside them.

var errNoNCM2MP3 = errors.New("ncm2mp3 not found; install it with: cargo install ncm2mp3")

// ncmCodecs maps the inner format reported by `ncm2mp3 info` to the
// extension ncm2mp3 writes and the codec name ffprobe would report.
var ncmCodecs = map[string]string{"mp3": "mp3", "flac": "flac", "m4a": "aac", "wav": "pcm_s16le", "ogg": "vorbis"}

func isNCM(path string) bool { return strings.EqualFold(filepath.Ext(path), ".ncm") }

// ncmArgs pins the language and ignores user config so output stays parseable.
func ncmArgs(args ...string) []string {
	return append([]string{"-L", "en", "--no-config", "--color", "never"}, args...)
}

// probeNCM builds a probe from `ncm2mp3 info` without decrypting the audio.
func probeNCM(path string) (*probe, error) {
	out, err := exec.Command("ncm2mp3", ncmArgs("info", safeInput(path))...).CombinedOutput()
	if errors.Is(err, exec.ErrNotFound) {
		return nil, errNoNCM2MP3
	}
	// ncm2mp3 can exit 0 on a per-file failure, so always look for "✗".
	if err := ncmError(out, err); err != nil {
		return nil, err
	}
	info := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), ": "); ok {
			info[k] = strings.TrimSpace(v)
		}
	}
	ext := strings.ToLower(info["Format"])
	codec, ok := ncmCodecs[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported NCM inner format %q", info["Format"])
	}

	p := &probe{NCM: ext}
	p.Format.Filename = path
	p.Format.FormatName = "ncm → " + ext
	p.Format.FormatLongName = "NetEase Cloud Music"
	if secs, ok := parseSecs(info["Duration"]); ok {
		p.Format.Duration = fmtSecs(secs)
	}
	if fi, err := os.Stat(path); err == nil {
		p.Format.Size = strconv.FormatInt(fi.Size(), 10)
	}
	p.Format.Tags = map[string]string{"title": info["Title"]}
	s := stream{CodecType: "audio", CodecName: codec}
	if kbps, err := strconv.Atoi(strings.TrimSuffix(info["Bitrate"], " kbps")); err == nil {
		s.BitRate = strconv.Itoa(kbps * 1000)
	}
	p.Streams = []stream{s}
	// ncm2mp3 embeds the cover after the audio, e.g. "Cover: JPEG, 35.64 KB".
	if kind, _, _ := strings.Cut(info["Cover"], ","); kind != "" {
		codec := map[string]string{"JPEG": "mjpeg", "PNG": "png"}[kind]
		p.Streams = append(p.Streams, stream{CodecType: "video", CodecName: codec, Disposition: map[string]int{"attached_pic": 1}})
	}
	return p, nil
}

// decryptNCM decrypts in into the empty directory dir and returns the
// decrypted file's path.
func decryptNCM(ctx context.Context, in, dir string) (string, error) {
	out, err := exec.CommandContext(ctx, "ncm2mp3", ncmArgs("-o", dir, safeInput(in))...).CombinedOutput()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if errors.Is(err, exec.ErrNotFound) {
		return "", errNoNCM2MP3
	}
	if err := ncmError(out, err); err != nil {
		return "", err
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(files) != 1 {
		return "", ncmError(out, errors.New("ncm2mp3 produced no output"))
	}
	return files[0], nil
}

// ncmError extracts ncm2mp3's per-file failure line ("✗ path: reason").
func ncmError(out []byte, err error) error {
	for line := range strings.Lines(string(out)) {
		if rest, ok := strings.CutPrefix(line, "✗ "); ok {
			if _, reason, ok := strings.Cut(rest, ": "); ok {
				return errors.New(strings.TrimSpace(reason))
			}
		}
	}
	return err
}
