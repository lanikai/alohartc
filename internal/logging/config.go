package logging

import (
	"fmt"
	"os"
	"strings"
)

const envVar = "LOGLEVEL"

var tagLevels []struct {
	tag   string
	level Level
}

func init() {
	// Parse environment variable into comma-separated "tag=level" directives.
	// If "tag=" is absent, use the level as the default.
	for _, d := range strings.Split(os.Getenv(envVar), ",") {
		if d == "" {
			continue
		}
		v := strings.SplitN(d, "=", 2)
		levelString := v[len(v)-1]
		if level, err := parseLevel(levelString); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid %s directive '%s': %s\n", envVar, d, err)
		} else {
			if len(v) == 1 {
				defaultLevel = level
			} else {
				tagLevels = append(tagLevels, struct {
					tag   string
					level Level
				}{v[0], level})
			}
		}
	}

	DefaultLogger.Level = defaultLevel
}

func determineLevel(tag string, fallback Level) Level {
	for _, e := range tagLevels {
		if e.tag == tag {
			return e.level
		}
	}
	return fallback
}
