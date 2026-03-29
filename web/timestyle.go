package web

import "time"

// DisplayTimeLayoutWeb 控制台 JSON 中人类可读时间格式（秒精度，按 Asia/Shanghai 展示）。
const DisplayTimeLayoutWeb = "2006-01-02 15:04:05"

var webDisplayTZ *time.Location

func init() {
	var err error
	webDisplayTZ, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		webDisplayTZ = time.FixedZone("CST", 8*3600)
	}
}

// FormatDisplayWeb 将任意时刻 t 转为控制台展示用字符串（中国标准时间 CST / Asia/Shanghai）；零值返回空串。
func FormatDisplayWeb(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(webDisplayTZ).Format(DisplayTimeLayoutWeb)
}

// FormatDisplayWebPtr 非零时间返回格式化指针，零值返回 nil（JSON omitempty 友好）。
func FormatDisplayWebPtr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := FormatDisplayWeb(t)
	return &s
}
