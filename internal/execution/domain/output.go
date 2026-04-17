package domain

import "strings"

func NormalizeOutput(raw string) string {
	normalizedLineBreaks := strings.ReplaceAll(raw, "\r\n", "\n")
	return strings.TrimRight(normalizedLineBreaks, " \t\n\r")
}
