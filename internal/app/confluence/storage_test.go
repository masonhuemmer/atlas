package confluence

import (
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestStorageBodyPreservesRunbookContent(t *testing.T) {
	input := "## Steps\n\nSet <FQDN> before running:\n\n```bash\necho '<FQDN>'\n```\n"
	got, err := StorageBody(input, domain.DefaultBodyFormat)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<h2>Steps</h2>", "&lt;FQDN&gt;", "<pre><code class=\"language-bash\">"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "## Steps") {
		t.Fatalf("unrendered Markdown: %q", got)
	}
}

func TestStorageBodyPassesPreparedConfluenceMarkup(t *testing.T) {
	input := `<ac:structured-macro ac:name="code"><ac:plain-text-body><![CDATA[echo hello]]></ac:plain-text-body></ac:structured-macro>`
	got, err := StorageBody(input, domain.StorageBodyFormat)
	if err != nil || got != input {
		t.Fatalf("got %q, error %v", got, err)
	}
}
