package main

import (
	"os"
	"strings"
)

type lang int

const (
	langEN lang = iota
	langZH
)

// zh maps English UI strings (used as keys) to Simplified Chinese.
var zh = map[string]string{
	"Info":     "信息",
	"Download": "下载",
	"Convert":  "转换",

	"tab/1-3 switch · L 中文 · q quit": "tab/1-3 切换 · L English · q 退出",
	"Usage: mx info <file>":          "用法：mx info <文件>",
	"Probing %s …":                   "正在读取 %s …",
	"(not implemented yet)":          "（尚未实现）",
	"args: ":                         "参数：",

	"General":      "概览",
	"Format":       "格式",
	"Duration":     "时长",
	"Size":         "大小",
	"Bitrate":      "码率",
	"Title":        "标题",
	"Codec":        "编码",
	"Resolution":   "分辨率",
	"Frame rate":   "帧率",
	"Pixel format": "像素格式",
	"Sample rate":  "采样率",
	"Channels":     "声道",
	"Language":     "语言",

	"Video":      "视频",
	"Audio":      "音频",
	"Subtitle":   "字幕",
	"Data":       "数据",
	"Attachment": "附件",
	"Unknown":    "未知",
}

// t translates an English UI string; unknown keys fall back to English.
func (l lang) t(s string) string {
	if l == langZH {
		if v, ok := zh[s]; ok {
			return v
		}
	}
	return s
}

// detectLang picks the initial UI language from the POSIX locale variables.
func detectLang() lang {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			if strings.HasPrefix(v, "zh") {
				return langZH
			}
			return langEN
		}
	}
	return langEN
}
