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

	"Usage: mx cv <file>... [format]":    "用法：mx cv <文件>... [格式]",
	"Probing %d file(s) …":               "正在读取 %d 个文件 …",
	"No target format fits these inputs": "没有适用于这些文件的目标格式",
	"Input":                              "输入",
	"%d files":                           "%d 个文件",
	"+%d more":                           "等 %d 个",
	"Quality":                            "质量",
	"Trim":                               "裁剪",
	"animated, no audio":                 "动图，无声",
	"⚡Copy":                              "⚡原样",
	"High":                               "高",
	"Medium":                             "标准",
	"Small":                              "小",
	"Lossless":                           "无损",
	"Original":                           "原始",
	"All":                                "全部",
	"First":                              "只留前",
	"Last":                               "只留后",
	"Output":                             "输出",
	"Command":                            "命令",
	"(same folder)":                      "（同目录）",

	"↑↓ option · ←→ change · enter start": "↑↓ 选项 · ←→ 切换 · enter 开始",
	" · 0-9 seconds":                                    " · 0-9 输入秒数",
	"Type seconds, e.g. 30 or 1:30":                     "输入秒数，如 30 或 1:30",
	"⚡ Copy streams: no re-encode, fast and lossless":   "⚡ 直接复制编码：不重编码，快且无损",
	" · cuts at keyframes, may be off by a few seconds": " · 按关键帧切，可能有几秒误差",
	"⚡ needs original resolution":                       "⚡原样需要保持原始分辨率",
	"⚡ unavailable: codecs don't fit this format":       "⚡原样不可用：编码与目标格式不兼容",

	"canceled": "已取消",
	"skipped":  "已跳过",
	"esc cancel (unfinished file is deleted)": "esc 取消（删除未完成的文件）",
	"left %s":                           "剩余 %s",
	"Done: %d succeeded, %d failed":     "完成：成功 %d 个，失败 %d 个",
	"Canceled: %d succeeded, %d failed": "已取消：成功 %d 个，失败 %d 个",
	"enter back · q quit":               "enter 返回 · q 退出",
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
