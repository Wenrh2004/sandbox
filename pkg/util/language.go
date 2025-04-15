package util

import "strings"

func DetectLanguage(filename string) string {
	switch {
	case strings.HasSuffix(filename, ".py"):
		return "python"
	case strings.HasSuffix(filename, ".cpp"):
		return "cpp"
	case strings.HasSuffix(filename, ".js"):
		return "node"
	case strings.HasSuffix(filename, ".java"):
		return "java"
	default:
		return ""
	}
}
