package atlassian

import (
	"context"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func (m *Memory) Comments(_ context.Context, hostname, key string, startAt, limit int) (domain.JiraCommentPage, error) {
	key = strings.ToUpper(strings.TrimSpace(key))
	m.mu.Lock()
	defer m.mu.Unlock()
	lookup := memKey(hostname, key)
	if _, ok := m.issues[lookup]; !ok {
		return domain.JiraCommentPage{}, domain.NotFound("issue not found")
	}
	items := m.comments[lookup]
	if startAt > len(items) {
		startAt = len(items)
	}
	end := startAt + limit
	if end > len(items) {
		end = len(items)
	}
	page := domain.JiraCommentPage{
		Key: key, Site: hostname, Total: len(items), StartAt: startAt, Limit: limit,
		Comments: append([]domain.JiraComment{}, items[startAt:end]...), IsLast: end == len(items),
	}
	if !page.IsLast {
		page.NextStartAt = end
	}
	return page, nil
}

func (m *Memory) GetComment(_ context.Context, hostname, key, id string) (domain.JiraComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	lookup := memKey(hostname, strings.ToUpper(strings.TrimSpace(key)))
	if _, ok := m.issues[lookup]; !ok {
		return domain.JiraComment{}, domain.NotFound("issue not found")
	}
	for _, comment := range m.comments[lookup] {
		if comment.ID == id {
			return comment, nil
		}
	}
	return domain.JiraComment{}, domain.NotFound("comment not found")
}

func (m *Memory) EditComment(_ context.Context, hostname, key, id, body string) (domain.JiraComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	lookup := memKey(hostname, strings.ToUpper(strings.TrimSpace(key)))
	iss, ok := m.issues[lookup]
	if !ok {
		return domain.JiraComment{}, domain.NotFound("issue not found")
	}
	for i := range m.comments[lookup] {
		if m.comments[lookup][i].ID == id {
			m.comments[lookup][i].Body = body
			iss.Comments[i] = body
			m.putLocked(iss)
			return m.comments[lookup][i], nil
		}
	}
	return domain.JiraComment{}, domain.NotFound("comment not found")
}

func (m *Memory) DeleteComment(_ context.Context, hostname, key, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	lookup := memKey(hostname, strings.ToUpper(strings.TrimSpace(key)))
	iss, ok := m.issues[lookup]
	if !ok {
		return domain.NotFound("issue not found")
	}
	for i := range m.comments[lookup] {
		if m.comments[lookup][i].ID == id {
			m.comments[lookup] = append(m.comments[lookup][:i], m.comments[lookup][i+1:]...)
			iss.Comments = append(iss.Comments[:i], iss.Comments[i+1:]...)
			m.putLocked(iss)
			return nil
		}
	}
	return domain.NotFound("comment not found")
}
