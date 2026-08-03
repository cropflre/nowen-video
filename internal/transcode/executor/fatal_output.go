package executor

import (
	"fmt"
	"strings"
)

const FatalOutputCodeENOSPC = "write_enospc"

type FatalOutputError struct {
	Code string
	Line string
}

func (e *FatalOutputError) Error() string {
	if strings.TrimSpace(e.Line) == "" {
		return fmt.Sprintf("fatal process output: %s", e.Code)
	}
	return fmt.Sprintf("fatal process output %s: %s", e.Code, e.Line)
}

func DetectFatalOutput(lines []string) (code string, line string, ok bool) {
	for _, candidate := range lines {
		lower := strings.ToLower(candidate)
		if strings.Contains(lower, "no space left on device") || strings.Contains(lower, "enospc") {
			return FatalOutputCodeENOSPC, candidate, true
		}
	}
	return "", "", false
}
