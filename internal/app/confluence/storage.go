package confluence

import (
	"bytes"
	stdhtml "html"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"github.com/masonhuemmer/atlas/internal/domain"
)

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(
		gmhtml.WithXHTML(),
		renderer.WithNodeRenderers(util.Prioritized(escapedHTML{}, 500)),
	),
)

// StorageBody converts Markdown to Confluence storage markup. Storage input is
// passed through for pages that already contain Confluence XHTML and macros.
func StorageBody(body, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", domain.DefaultBodyFormat:
		var out bytes.Buffer
		if err := markdown.Convert([]byte(body), &out); err != nil {
			return "", domain.Service(err.Error())
		}
		return out.String(), nil
	case domain.StorageBodyFormat:
		return body, nil
	default:
		return "", domain.Usage("body-format must be markdown or storage")
	}
}

// Markdown raw HTML is displayed as text; it must not disappear from runbook
// placeholders such as <FQDN>. Use body-format=storage for intended XHTML.
type escapedHTML struct{}

func (escapedHTML) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindRawHTML, renderEscapedRawHTML)
	reg.Register(ast.KindHTMLBlock, renderEscapedHTMLBlock)
}

func renderEscapedRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	segments := node.(*ast.RawHTML).Segments
	for i := 0; i < segments.Len(); i++ {
		segment := segments.At(i)
		if _, err := w.WriteString(stdhtml.EscapeString(string(segment.Value(source)))); err != nil {
			return ast.WalkStop, err
		}
	}
	return ast.WalkSkipChildren, nil
}

func renderEscapedHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	block := node.(*ast.HTMLBlock)
	if entering {
		for i := 0; i < block.Lines().Len(); i++ {
			line := block.Lines().At(i)
			if _, err := w.WriteString(stdhtml.EscapeString(string(line.Value(source)))); err != nil {
				return ast.WalkStop, err
			}
		}
	} else if block.HasClosure() {
		if _, err := w.WriteString(stdhtml.EscapeString(string(block.ClosureLine.Value(source)))); err != nil {
			return ast.WalkStop, err
		}
	}
	return ast.WalkContinue, nil
}
