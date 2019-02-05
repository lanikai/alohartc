package media

import (
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// Open a source based on its "source spec". A source spec is a colon-separated string
// consisting of a source tag and a source path:
//    sourceSpec = sourceTag + ":" + sourcePath
// The format of the source path is defined by the registered OpenFunc.
func OpenSource(spec string) (Source, error) {
	// Log known source types, for debug purposes.
	var tags []string
	for t, _ := range registry {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	log.Debug("Registered source types: %v", tags)

	// Split the spec string into tag and path
	parts := strings.SplitN(spec, ":", 2)
	var tag, path string
	tag = parts[0]
	if len(parts) == 2 {
		path = parts[1]
	}

	if open, found := registry[tag]; found {
		return open(path)
	} else {
		return nil, errors.Errorf("Source type '%s' not registered", tag)
	}
}

// A function used to open a specific source type.
type OpenFunc func(path string) (Source, error)

var registry = map[string]OpenFunc{}

// Register a source type, identified by its "source tag". Sources of this type will be
// opened with the given function.
func RegisterSourceType(tag string, open OpenFunc) {
	registry[tag] = open
}
