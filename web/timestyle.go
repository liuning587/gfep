package web

import "time"

// DisplayTimeLayoutUTC Web 与相关 API 对人类展示的统一时间格式（UTC，秒精度）。
const DisplayTimeLayoutUTC = "2006-01-02 15:04:05"

// FormatDisplayUTC 将 t 转为 UTC 下的可读字符串；零值返回空串。
func FormatDisplayUTC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(DisplayTimeLayoutUTC)
}

// FormatDisplayUTCPtr 非零时间返回格式化指针，零值返回 nil（JSON omitempty 友好）。
func FormatDisplayUTCPtr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := FormatDisplayUTC(t)
	return &s
}
