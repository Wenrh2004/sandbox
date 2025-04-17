package str

import "strings"

func RemoveSpace(s string) string {
	return strings.ReplaceAll(s, space, noSpace)
}
