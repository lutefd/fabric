package fabric

import (
	"errors"
	"strconv"
	"strings"
)

func emptyAsNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(none)"
	}
	return value
}

func emptyAsUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

func sourceThread(threadID string) string {
	if threadID == "" {
		return "note"
	}
	return "note from " + threadID
}

func parseBudget(value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.New("--budget must be a number")
	}
	if parsed <= 0 {
		return 0, errors.New("--budget must be positive")
	}
	return parsed, nil
}

func flagsFirst(args []string) []string {
	var flags []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}

type stringListFlag []string

func (s *stringListFlag) String() string {
	return strings.Join(*s, ",")
}

func (s stringListFlag) StringOrNone() string {
	if len(s) == 0 {
		return "(none)"
	}
	return strings.Join(s, ", ")
}

func (s *stringListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("empty value")
	}
	*s = append(*s, value)
	return nil
}
