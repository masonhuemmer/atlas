package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// flagName is a long flag name without dashes. Anything else could end flag
// parsing early or set a flag the caller does not own, such as dry-run=false.
var flagName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func FlagMapToArgs(ns, verb string, pos []string, flags map[string]any) ([]string, error) {
	out := []string{"atlas", ns, verb}
	for _, p := range pos {
		if strings.HasPrefix(p, "-") {
			return nil, &domain.Error{Class: domain.ClassUsage, Message: fmt.Sprintf("positional arg %q looks like a flag", p), Hint: "pass flags in flags"}
		}
	}
	out = append(out, pos...)
	if len(flags) == 0 {
		return out, nil
	}
	keys := make([]string, 0, len(flags))
	for k := range flags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !flagName.MatchString(k) {
			return nil, domain.Usagef("unknown flag %q", k)
		}
		// One --name=value token per value, booleans included: a value such as
		// "--json" cannot be peeled as a global flag, and no bare --name can
		// take the next token (the injected --dry-run) as its value.
		name := "--" + k
		switch v := flags[k].(type) {
		case nil:
		case bool:
			if v {
				out = append(out, name+"=true")
			}
		case string:
			out = append(out, name+"="+v)
		case float64:
			if v == float64(int64(v)) {
				out = append(out, name+"="+strconv.FormatInt(int64(v), 10))
			} else {
				out = append(out, name+"="+strconv.FormatFloat(v, 'f', -1, 64))
			}
		case json.Number:
			out = append(out, name+"="+v.String())
		case []string:
			for _, s := range v {
				out = append(out, name+"="+s)
			}
		case []any:
			for _, e := range v {
				s, ok := e.(string)
				if !ok {
					return nil, domain.Usagef("flag %s values must be strings", k)
				}
				out = append(out, name+"="+s)
			}
		default:
			return nil, domain.Usagef("unsupported flag type for %s", k)
		}
	}
	return out, nil
}

func applyWriteGate(ns, verb string, flags map[string]any, optIn bool) map[string]any {
	if !isWrite(ns, verb) || optIn {
		return flags
	}
	n := map[string]any{"dry-run": true}
	for k, v := range flags {
		n[k] = v
	}
	n["dry-run"] = true
	return n
}

func isWrite(ns, verb string) bool {
	switch ns + " " + verb {
	case "jira create", "jira edit", "jira comment", "jira transition", "jira link",
		"confluence create", "confluence update", "confluence move",
		"pr create", "pr edit", "pr comment", "pr merge",
		"jsm create", "jsm comment", "jsm transition":
		return true
	}
	return false
}

// isRead is an allowlist. A verb it does not name is refused by atlas_read,
// so a new write verb cannot run through the read-only tool.
func isRead(ns, verb string) bool {
	switch ns + " " + verb {
	case "auth status", "site list", "site resolve",
		"jira get", "jira search", "jira users",
		"confluence get", "confluence search",
		"pr get", "pr list", "pr diff",
		"jsm desks", "jsm types", "jsm list", "jsm get":
		return true
	}
	return false
}

func runForbidden(ns, verb string) error {
	if ns == "mcp" {
		return &domain.Error{Class: domain.ClassUsage, Message: "mcp is not a run namespace", Hint: "use atlas_status, atlas_help, atlas_read, or atlas_write"}
	}
	if ns == "auth" && (verb == "login" || verb == "logout") {
		return &domain.Error{Class: domain.ClassUsage, Message: "auth login is a human terminal command", Hint: "run atlas auth login in a terminal"}
	}
	return nil
}

func checkRun(ns, verb string) error {
	if ns == "" || verb == "" {
		return domain.Usage("namespace and verb are required")
	}
	return runForbidden(ns, verb)
}

func buildReadArgs(ns, verb string, pos []string, flags map[string]any) ([]string, error) {
	if err := checkRun(ns, verb); err != nil {
		return nil, err
	}
	if !isRead(ns, verb) {
		hint := "atlas_help lists each namespace's verbs"
		if isWrite(ns, verb) {
			hint = "use atlas_write"
		}
		return nil, &domain.Error{Class: domain.ClassUsage, Message: fmt.Sprintf("%s %s is not a read", ns, verb), Hint: hint}
	}
	return FlagMapToArgs(ns, verb, pos, flags)
}

func buildWriteArgs(ns, verb string, pos []string, flags map[string]any, optIn bool) ([]string, error) {
	if err := checkRun(ns, verb); err != nil {
		return nil, err
	}
	if !isWrite(ns, verb) {
		hint := "atlas_help lists each namespace's verbs"
		if isRead(ns, verb) {
			hint = "use atlas_read"
		}
		return nil, &domain.Error{Class: domain.ClassUsage, Message: fmt.Sprintf("%s %s is not a write", ns, verb), Hint: hint}
	}
	return FlagMapToArgs(ns, verb, pos, applyWriteGate(ns, verb, flags, optIn))
}
