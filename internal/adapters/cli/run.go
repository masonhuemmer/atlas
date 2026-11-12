package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/masonhuemmer/atlas/internal/app/auth"
	"github.com/masonhuemmer/atlas/internal/app/confluence"
	"github.com/masonhuemmer/atlas/internal/app/jira"
	"github.com/masonhuemmer/atlas/internal/app/jsm"
	"github.com/masonhuemmer/atlas/internal/app/pipeline"
	"github.com/masonhuemmer/atlas/internal/app/pr"
	"github.com/masonhuemmer/atlas/internal/config"
	"github.com/masonhuemmer/atlas/internal/domain"
)

type Deps struct {
	Config     config.Config
	Store      auth.Store
	Login      auth.LoginFn
	Jira       jira.Store
	Confluence confluence.Store
	PR         pr.Store
	Pipeline   pipeline.Store
	JSM        jsm.Store
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
}

func Run(args []string, d Deps) int {
	if d.Stdin == nil {
		d.Stdin = os.Stdin
	}
	if d.Stdout == nil {
		d.Stdout = os.Stdout
	}
	if d.Stderr == nil {
		d.Stderr = os.Stderr
	}
	human, jsonOn, verbose, rest := peelGlobals(args)
	if human && jsonOn {
		return fail(d, domain.Usage("use only one of --json or --human"))
	}
	format := "json"
	if human {
		format = "human"
	}
	if len(rest) == 0 || rest[0] == "help" {
		return writeHelp(d.Stdout, rootHelp)
	}
	ns, rest := rest[0], rest[1:]
	switch ns {
	case "auth":
		return runAuth(rest, d, format, verbose)
	case "site":
		return runSite(rest, d, format)
	case "jira":
		return runJira(rest, d, format)
	case "confluence":
		return runConfluence(rest, d, format)
	case "pr":
		return runPR(rest, d, format)
	case "pipeline":
		return runPipeline(rest, d, format)
	case "jsm":
		return runJSM(rest, d, format)
	case "mcp":
		return runMCP(rest, d, format)
	default:
		if strings.HasPrefix(ns, "-") {
			return writeHelp(d.Stdout, rootHelp)
		}
		return fail(d, domain.Usagef("unknown namespace %q", ns))
	}
}

func peelGlobals(args []string) (human, jsonOn, verbose bool, rest []string) {
	if len(args) > 0 {
		base := args[0]
		if strings.HasSuffix(base, "atlas") || strings.HasSuffix(base, "atlas.exe") || strings.Contains(base, "/") {
			args = args[1:]
		}
	}
	sawCmd := false
	for _, a := range args {
		switch a {
		case "--human":
			human = true
		case "--json":
			jsonOn = true
		case "--verbose", "--debug":
			verbose = true
		case "--help", "-h":
			if !sawCmd {
				rest = append(rest, "help")
			} else {
				rest = append(rest, a)
			}
		default:
			if !strings.HasPrefix(a, "-") {
				sawCmd = true
			}
			rest = append(rest, a)
		}
	}
	return
}

// Fail writes a {class,message,hint} JSON object to stderr. Used by main when config.Load fails.
func Fail(d Deps, err error) int {
	if d.Stderr == nil {
		d.Stderr = os.Stderr
	}
	return fail(d, err)
}

func fail(d Deps, err error) int {
	code := domain.ExitOf(err)
	cls := domain.ClassOf(err)
	msg := err.Error()
	hint := ""
	if de, ok := err.(*domain.Error); ok {
		hint = de.Hint
	}
	_ = json.NewEncoder(d.Stderr).Encode(errObj{Class: cls, Message: redact(msg), Hint: hint})
	return code
}

type errObj struct {
	Class   string `json:"class"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func success(d Deps, format string, v any) int {
	if format == "human" {
		fmt.Fprintln(d.Stdout, humanize(v))
		return domain.ExitOK
	}
	enc := json.NewEncoder(d.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	return domain.ExitOK
}

func writeHelp(w io.Writer, s string) int {
	fmt.Fprint(w, s)
	if !strings.HasSuffix(s, "\n") {
		fmt.Fprintln(w)
	}
	return domain.ExitOK
}

func humanize(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
	return strings.TrimSpace(buf.String())
}

func ctx() context.Context { return context.Background() }

func hasHelp(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

var boolFlags = map[string]bool{
	"--dry-run": true, "--help": true, "-h": true,
	"--json": true, "--human": true, "--verbose": true, "--debug": true,
	"--from-op": true,
	"--secured": true, "--enabled": true,
}

func parseMixed(fsset *flag.FlagSet, args []string) error {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			if strings.Contains(a, "=") || boolFlags[a] {
				continue
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	return fsset.Parse(append(flags, pos...))
}
