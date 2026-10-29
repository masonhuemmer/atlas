package rest

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// maxWriteErrorText caps the Jira validation text added to a failed write.
const maxWriteErrorText = 500

// ConvertCustomFields returns a copy of fields in which each customfield_*
// string whose schema type is textarea is converted from Markdown to ADF.
// Jira REST v3 rejects a plain string for those with a 400. Every other
// value, and every other field, is returned as given. catalog is called at
// most once, and only when a customfield_* string is present, so a write with
// nothing to convert makes no extra request.
func ConvertCustomFields(fields map[string]any, catalog FieldCatalog) (map[string]any, error) {
	out := make(map[string]any, len(fields))
	needsSchema := false
	for name, value := range fields {
		out[name] = value
		if _, isString := value.(string); isString && customFieldID.MatchString(name) {
			needsSchema = true
		}
	}
	if !needsSchema || catalog == nil {
		return out, nil
	}
	catalogue, err := catalog()
	if err != nil {
		return nil, err
	}
	for name, value := range fields {
		if text, isString := value.(string); isString && customFieldID.MatchString(name) && catalogue[name].Type == "textarea" {
			out[name] = markdownToADF(text)
		}
	}
	return out, nil
}

// prepareWriteFields is the one conversion create and edit share: a
// top-level description string and each textarea custom field string are
// Markdown and become ADF.
func prepareWriteFields(fields map[string]any, catalog FieldCatalog) (map[string]any, error) {
	out, err := ConvertCustomFields(fields, catalog)
	if err != nil {
		return nil, err
	}
	if description, ok := out["description"].(string); ok {
		out["description"] = markdownToADF(description)
	}
	return out, nil
}

// markdownToADF is adfFromMarkdown with an untyped nil for empty input, which
// marshals to null and clears the field.
func markdownToADF(markdown string) any {
	if doc := adfFromMarkdown(markdown); doc != nil {
		return doc
	}
	return nil
}

// CreateFields builds the `fields` object a create sends: the --fields
// extras with Markdown converted, then the values the flags own on top, so
// project, issuetype and summary cannot be overridden. The fake adapter
// shares this with the live one so a dry-run shows the real payload.
func CreateFields(in domain.CreateIssue, catalog FieldCatalog) (map[string]any, error) {
	fields, err := prepareWriteFields(in.Fields, catalog)
	if err != nil {
		return nil, err
	}
	fields["project"] = map[string]any{"key": in.Project}
	fields["issuetype"] = map[string]any{"name": in.IssueType}
	fields["summary"] = in.Summary
	if in.Parent != "" {
		fields["parent"] = map[string]any{"key": in.Parent}
	}
	if in.Description != "" {
		fields["description"] = adfFromMarkdown(in.Description)
	}
	if len(in.Labels) > 0 {
		fields["labels"] = in.Labels
	}
	if in.Assignee != "" {
		fields["assignee"] = map[string]any{"accountId": in.Assignee}
	}
	return fields, nil
}

// jiraWriteError maps a failed create or edit. A 400 carries Jira's own
// validation text, which names the rejected field and why, so it is kept in
// the message. The class stays service.
func jiraWriteError(code int, body []byte) error {
	if code == http.StatusBadRequest {
		if detail := jiraValidationText(body); detail != "" {
			return domain.Service("jira service error (400): " + detail)
		}
	}
	return MapStatus(code)
}

// jiraValidationText joins errorMessages and then errors (by field key) from
// a Jira error body, capped at maxWriteErrorText characters.
func jiraValidationText(body []byte) string {
	var parsed map[string]any
	if json.Unmarshal(body, &parsed) != nil {
		return ""
	}
	var parts []string
	for _, raw := range asList(parsed["errorMessages"]) {
		if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
			parts = append(parts, strings.TrimSpace(text))
		}
	}
	byField := asMap(parsed["errors"])
	names := make([]string, 0, len(byField))
	for name := range byField {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if text, ok := byField[name].(string); ok && strings.TrimSpace(text) != "" {
			parts = append(parts, name+": "+strings.TrimSpace(text))
		}
	}
	detail := strings.Join(parts, "; ")
	if utf8.RuneCountInString(detail) > maxWriteErrorText {
		detail = string([]rune(detail)[:maxWriteErrorText]) + "…"
	}
	return detail
}
