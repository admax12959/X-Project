package domain

import "regexp"

// Username format (spec #2, 数据模型要点).
var usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{1,31}$`)
