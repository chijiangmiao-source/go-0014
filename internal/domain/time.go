package domain

import (
	"strings"
	"time"
)

func ParseRFC3339NanoUTC(text string) (int64, error) {
	if !strings.Contains(text, "T") {
		return 0, NewInputError(CodeInvalidInput, "time must use RFC3339Nano", text)
	}
	if !strings.HasSuffix(text, "Z") {
		t := text[strings.LastIndex(text, "T")+1:]
		if !strings.ContainsAny(t, "+-") {
			return 0, NewInputError(CodeInvalidInput, "time must include Z or explicit offset", text)
		}
	}
	if strings.Contains(text, ":60") {
		return 0, NewInputError(CodeInvalidInput, "leap second text is not accepted", text)
	}
	parsed, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return 0, err
	}
	return parsed.UTC().UnixNano(), nil
}

func FormatUTC(nanos int64) string {
	return time.Unix(0, nanos).UTC().Format(time.RFC3339Nano)
}
