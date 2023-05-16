package util

import (
	"fmt"
	"regexp"
)

func RegexNamedMatches(r *regexp.Regexp, s string) (map[string]string, error) {
	match := r.FindStringSubmatch(s)
	if match == nil {
		return nil, fmt.Errorf("expected regex match of %q on string: %q", r.String(), s)
	}
	m := make(map[string]string)
	for _, name := range r.SubexpNames() {
		if name == "" {
			continue
		}
		i := r.SubexpIndex(name)
		if i == -1 {
			return nil, fmt.Errorf(
				"could not find index of capture group %q in match: regex = %q, s = %q",
				name,
				r.String(),
				s,
			)
		}
		m[name] = match[i]
	}
	return m, nil
}

func Linkify(text, url string) string {
	// See https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
	// for more information about this escape sequence.
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}
