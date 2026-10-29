package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/masonhuemmer/atlas/internal/app/auth"
	"github.com/masonhuemmer/atlas/internal/domain"
)

// FieldMeta is the display name and schema type of one custom field.
type FieldMeta struct {
	Name string
	Type string
}

// FieldCatalog loads a site's custom field metadata. Code that takes one
// calls it at most once, and only when it needs the answer, so a request
// that never touches a custom field makes no extra call.
type FieldCatalog func() (map[string]FieldMeta, error)

var customFieldID = regexp.MustCompile(`^customfield_\d+$`)

// IsCustomFieldID reports whether name is a REST custom field key.
func IsCustomFieldID(name string) bool { return customFieldID.MatchString(name) }

// maxFieldCatalogBody is the read cap for GET /field, which grows with the
// number of fields on the site and has no paging.
const maxFieldCatalogBody = 8 << 20

// changelogPageSize is the maxResults asked of the changelog endpoint. The
// server may serve fewer, so paging advances by the count it returned.
const changelogPageSize = 100

// GetCustom reads the custom fields and status history that Get leaves out.
// ids selects specific custom fields and all selects every non-empty one;
// empty and absent fields are omitted from the result.
func (j Jira) GetCustom(ctx context.Context, hostname, key string, ids []string, all, history bool) (domain.IssueCustom, error) {
	var out domain.IssueCustom
	key = strings.ToUpper(strings.TrimSpace(key))
	ids = uniqueStrings(ids)
	for _, id := range ids {
		if !customFieldID.MatchString(id) {
			return out, domain.Usagef("invalid custom field id %q", id).WithHint("use customfield_ followed by digits")
		}
	}
	cred, err := j.credForHost(hostname)
	if err != nil {
		return out, err
	}
	if all || len(ids) > 0 {
		requested := "*all,-comment,-worklog,-attachment"
		if !all {
			requested = strings.Join(ids, ",")
		}
		var issue map[string]any
		u := joinURL(j.origin(hostname), "/rest/api/3/issue/"+q(key)+"?fields="+requested)
		if err := j.getJSON(ctx, u, cred, maxJSONBody, &issue); err != nil {
			return out, err
		}
		catalog, err := j.fieldCatalog(ctx, hostname)()
		if err != nil {
			return out, err
		}
		out.CustomFields = BuildCustomFields(asMap(issue["fields"]), catalog, ids, all)
	}
	if history {
		out.StatusHistory, err = j.statusHistory(ctx, hostname, key, cred)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// fieldCatalog returns a lazy loader for GET /rest/api/3/field.
func (j Jira) fieldCatalog(ctx context.Context, hostname string) FieldCatalog {
	return func() (map[string]FieldMeta, error) {
		cred, err := j.credForHost(hostname)
		if err != nil {
			return nil, err
		}
		var fields []map[string]any
		if err := j.getJSON(ctx, joinURL(j.origin(hostname), "/rest/api/3/field"), cred, maxFieldCatalogBody, &fields); err != nil {
			return nil, err
		}
		catalog := make(map[string]FieldMeta, len(fields))
		for _, field := range fields {
			if id := str(field, "id"); customFieldID.MatchString(id) {
				catalog[id] = FieldMeta{Name: str(field, "name"), Type: fieldTypeFromSchema(asMap(field["schema"]))}
			}
		}
		return catalog, nil
	}
}

// statusHistory returns every status transition in the issue changelog,
// oldest first, paging until the changelog is exhausted.
func (j Jira) statusHistory(ctx context.Context, hostname, key string, cred auth.Cred) ([]domain.StatusChange, error) {
	var changes []domain.StatusChange
	base := joinURL(j.origin(hostname), "/rest/api/3/issue/"+q(key)+"/changelog")
	for startAt := 0; ; {
		var page map[string]any
		u := base + "?startAt=" + strconv.Itoa(startAt) + "&maxResults=" + strconv.Itoa(changelogPageSize)
		if err := j.getJSON(ctx, u, cred, maxJSONBody, &page); err != nil {
			return nil, err
		}
		values := asList(page["values"])
		for _, rawEntry := range values {
			entry := asMap(rawEntry)
			for _, rawItem := range asList(entry["items"]) {
				item := asMap(rawItem)
				if str(item, "field") != "status" {
					continue
				}
				changes = append(changes, domain.StatusChange{
					Created: str(entry, "created"),
					Author:  displayUser(asMap(entry["author"])),
					From:    str(item, "fromString"),
					To:      str(item, "toString"),
				})
			}
		}
		startAt += len(values)
		isLast, hasLast := page["isLast"].(bool)
		total, hasTotal := page["total"].(float64)
		if len(values) == 0 || (hasLast && isLast) || (hasTotal && float64(startAt) >= total) || (!hasLast && !hasTotal) {
			return changes, nil
		}
	}
}

// getJSON GETs rawURL and decodes a complete 200 response into dst. A body
// cut by the read cap or a dropped connection is a service error: decoding
// it would yield an empty or partial result that looks like real data.
func (j Jira) getJSON(ctx context.Context, rawURL string, cred auth.Cred, limit int64, dst any) error {
	code, body, complete, err := j.doJSONLimit(ctx, http.MethodGet, rawURL, cred, nil, nil, limit)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return MapStatus(code)
	}
	if !complete {
		return domain.Service(fmt.Sprintf("jira response was incomplete: over the %d MiB read limit or the connection ended early", limit>>20))
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return domain.Service("jira returned invalid JSON")
	}
	return nil
}

// BuildCustomFields turns the custom entries of an issue's fields object
// into output rows. With all, every non-empty custom field is returned in
// ascending id order; otherwise only ids, in request order. Empty, null and
// absent fields are left out. The fake adapter shares this with the live one.
func BuildCustomFields(values map[string]any, catalog map[string]FieldMeta, ids []string, all bool) []domain.CustomField {
	var order []string
	if all {
		for id := range values {
			if customFieldID.MatchString(id) {
				order = append(order, id)
			}
		}
		sort.Slice(order, func(a, b int) bool {
			if len(order[a]) != len(order[b]) {
				return len(order[a]) < len(order[b])
			}
			return order[a] < order[b]
		})
	} else {
		order = uniqueStrings(ids)
	}
	var out []domain.CustomField
	for _, id := range order {
		text := flattenField(values[id])
		if text == "" {
			continue
		}
		meta := catalog[id]
		if meta.Name == "" {
			meta.Name = id
		}
		out = append(out, domain.CustomField{ID: id, Name: meta.Name, Type: meta.Type, Text: text})
	}
	return out
}

// fieldTypeFromSchema names a field's type: the last segment of schema.custom
// (customfieldtypes:textarea gives textarea), else the schema type.
func fieldTypeFromSchema(schema map[string]any) string {
	if custom := str(schema, "custom"); custom != "" {
		if i := strings.LastIndex(custom, ":"); i >= 0 {
			custom = custom[i+1:]
		}
		if custom != "" {
			return custom
		}
	}
	return str(schema, "type")
}

// flattenField renders any Jira field value as plain text; "" means empty.
// Atlassian Document Format becomes text with a newline between blocks, users
// their display name, options their value (a cascading child as "parent /
// child"), lists their members joined by "; ", and scalars as they are.
func flattenField(v any) string {
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return ""
		}
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case []any:
		parts := make([]string, 0, len(t))
		for _, element := range t {
			if text := flattenField(element); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		return flattenObject(t)
	default:
		return ""
	}
}

func flattenObject(m map[string]any) string {
	if str(m, "type") == "doc" {
		return strings.TrimSpace(adfPlain(m))
	}
	if value, ok := m["value"]; ok {
		text := flattenField(value)
		if child := flattenField(m["child"]); child != "" {
			if text == "" {
				return child
			}
			return text + " / " + child
		}
		return text
	}
	for _, key := range []string{"displayName", "accountId", "name"} {
		if text := str(m, key); text != "" {
			return text
		}
	}
	if len(m) == 0 {
		return ""
	}
	// An unrecognised shape is shown as compact JSON rather than dropped.
	encoded, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// adfInlineNodes are the ADF node types that sit inside a block's line of text.
var adfInlineNodes = map[string]bool{
	"text": true, "hardBreak": true, "mention": true, "emoji": true, "status": true,
	"inlineCard": true, "date": true, "placeholder": true, "mediaInline": true, "inlineExtension": true,
}

// adfPlain flattens an ADF node: inline nodes run together, blocks are
// separated by a newline, and empty blocks are dropped. This differs from
// textFromADF, which glues paragraphs together.
func adfPlain(node map[string]any) string {
	switch str(node, "type") {
	case "text":
		return adfLinkedText(node)
	case "hardBreak":
		return "\n"
	case "mention", "emoji", "status", "placeholder":
		attrs := asMap(node["attrs"])
		if text := str(attrs, "text"); text != "" {
			return text
		}
		return str(attrs, "shortName")
	case "date":
		return adfDateText(asMap(node["attrs"])["timestamp"])
	case "inlineCard", "blockCard", "embedCard":
		attrs := asMap(node["attrs"])
		if url := str(attrs, "url"); url != "" {
			return url
		}
		// A smart link may carry its JSON-LD in attrs.data instead of a url.
		return str(asMap(attrs["data"]), "url")
	}
	var lines []string
	var inline strings.Builder
	flush := func() {
		if inline.Len() > 0 {
			lines = append(lines, inline.String())
			inline.Reset()
		}
	}
	for _, rawChild := range asList(node["content"]) {
		child := asMap(rawChild)
		if adfInlineNodes[str(child, "type")] {
			inline.WriteString(adfPlain(child))
			continue
		}
		flush()
		if text := adfPlain(child); text != "" {
			lines = append(lines, text)
		}
	}
	flush()
	return strings.Join(lines, "\n")
}

// adfDateText renders a date node's attrs.timestamp, epoch milliseconds as a
// string or a number, as a UTC YYYY-MM-DD date. A value that is not a
// timestamp is kept as given rather than dropped.
func adfDateText(timestamp any) string {
	switch t := timestamp.(type) {
	case string:
		ms, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			return t
		}
		return time.UnixMilli(ms).UTC().Format("2006-01-02")
	case float64:
		return time.UnixMilli(int64(t)).UTC().Format("2006-01-02")
	}
	return ""
}

// adfLinkedText keeps a link mark as "text <href>"; text that is already the
// URL is not repeated.
func adfLinkedText(node map[string]any) string {
	text := str(node, "text")
	for _, rawMark := range asList(node["marks"]) {
		mark := asMap(rawMark)
		if str(mark, "type") != "link" {
			continue
		}
		href := str(asMap(mark["attrs"]), "href")
		switch {
		case href == "" || href == text:
			return text
		case text == "":
			return href
		default:
			return text + " <" + href + ">"
		}
	}
	return text
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
