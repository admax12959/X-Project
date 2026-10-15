package domain

import (
	"regexp"
	"strings"
)

// Username format (spec #2, 数据模型要点).
var usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{1,31}$`)

// Application key format (spec #2): lowercase slug, 2-40 characters.
var applicationKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)

var branchPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

// validBranchName is a conservative check for a Git branch name: no blanks, no
// "..", no empty path segments, no trailing "/", "." or ".lock". Used for the
// mainline branch and for the branches of maintenance lines.
func validBranchName(b string) bool {
	return len(b) <= 255 && branchPattern.MatchString(b) &&
		!strings.Contains(b, "..") && !strings.Contains(b, "//") &&
		!strings.HasSuffix(b, "/") && !strings.HasSuffix(b, ".") && !strings.HasSuffix(b, ".lock")
}
