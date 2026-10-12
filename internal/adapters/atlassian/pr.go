package atlassian

import (
	"context"
	"sort"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// PRAPI is the in-process Bitbucket adapter used by ATLAS_FAKE and tests.
type PRAPI struct {
	Memory *Memory
}

func (p PRAPI) Get(ctx context.Context, workspace, repo string, id int) (domain.PullRequest, error) {
	if p.Memory == nil {
		return domain.PullRequest{}, domain.Service("pr memory not configured")
	}
	return p.Memory.GetPR(ctx, workspace, repo, id)
}

func (p PRAPI) Comments(ctx context.Context, workspace, repo string, id, page, limit int) (domain.PullRequestCommentPage, error) {
	if p.Memory == nil {
		return domain.PullRequestCommentPage{}, domain.Service("pr memory not configured")
	}
	return p.Memory.CommentsPR(ctx, workspace, repo, id, page, limit)
}

func (p PRAPI) List(ctx context.Context, workspace, repo string) (domain.PullRequestList, error) {
	if p.Memory == nil {
		return domain.PullRequestList{}, domain.Service("pr memory not configured")
	}
	return p.Memory.ListPRs(ctx, workspace, repo)
}

func (p PRAPI) Create(ctx context.Context, workspace, repo string, in domain.CreatePullRequest, dryRun bool) (domain.PullRequest, error) {
	if p.Memory == nil {
		return domain.PullRequest{}, domain.Service("pr memory not configured")
	}
	return p.Memory.CreatePR(ctx, workspace, repo, in, dryRun)
}

func (p PRAPI) Edit(ctx context.Context, workspace, repo string, id int, in domain.EditPullRequest, dryRun bool) (domain.PullRequest, error) {
	if p.Memory == nil {
		return domain.PullRequest{}, domain.Service("pr memory not configured")
	}
	return p.Memory.EditPR(ctx, workspace, repo, id, in, dryRun)
}

func (p PRAPI) Comment(ctx context.Context, workspace, repo string, id int, body string, dryRun bool) error {
	if p.Memory == nil {
		return domain.Service("pr memory not configured")
	}
	return p.Memory.CommentPR(ctx, workspace, repo, id, body, dryRun)
}

func (p PRAPI) Approve(ctx context.Context, workspace, repo string, id int, dryRun bool) (domain.PullRequestApproval, error) {
	if p.Memory == nil {
		return domain.PullRequestApproval{}, domain.Service("pr memory not configured")
	}
	return p.Memory.ApprovePR(ctx, workspace, repo, id, dryRun)
}

func (p PRAPI) Merge(ctx context.Context, workspace, repo string, id int, dryRun bool) (domain.PullRequest, error) {
	if p.Memory == nil {
		return domain.PullRequest{}, domain.Service("pr memory not configured")
	}
	return p.Memory.MergePR(ctx, workspace, repo, id, dryRun)
}

func (p PRAPI) Diff(ctx context.Context, workspace, repo string, id int) (domain.PullRequestDiff, error) {
	if p.Memory == nil {
		return domain.PullRequestDiff{}, domain.Service("pr memory not configured")
	}
	return p.Memory.DiffPR(ctx, workspace, repo, id)
}

func (m *Memory) GetPR(_ context.Context, workspace, repo string, id int) (domain.PullRequest, error) {
	if m == nil {
		return domain.PullRequest{}, domain.Service("pr memory not configured")
	}
	workspace, repo, err := normalizePR(workspace, repo)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if id <= 0 {
		return domain.PullRequest{}, domain.Usage("pr id is required").WithHint("atlas pr get --repo atlas --id 1")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	pr, ok := m.prs[prKey(workspace, repo, id)]
	if !ok {
		return domain.PullRequest{}, domain.NotFound("pull request not found").WithHint("check --repo and --id")
	}
	return clonePR(pr), nil
}

func (m *Memory) CommentsPR(ctx context.Context, workspace, repo string, id, page, limit int) (domain.PullRequestCommentPage, error) {
	pr, err := m.GetPR(ctx, workspace, repo, id)
	if err != nil {
		return domain.PullRequestCommentPage{}, err
	}
	result := domain.PullRequestCommentPage{ID: id, Workspace: pr.Workspace, Repo: pr.Repo, Comments: []domain.PullRequestComment{}}
	start := (page - 1) * limit
	if start >= len(pr.Comments) {
		return result, nil
	}
	end := start + limit
	if end > len(pr.Comments) {
		end = len(pr.Comments)
	}
	for i := start; i < end; i++ {
		result.Comments = append(result.Comments, domain.PullRequestComment{ID: i + 1, Body: pr.Comments[i]})
	}
	if end < len(pr.Comments) {
		result.NextPage = page + 1
	}
	return result, nil
}

func (m *Memory) ListPRs(_ context.Context, workspace, repo string) (domain.PullRequestList, error) {
	if m == nil {
		return domain.PullRequestList{}, domain.Service("pr memory not configured")
	}
	workspace, repo, err := normalizePR(workspace, repo)
	if err != nil {
		return domain.PullRequestList{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]domain.PullRequest, 0)
	for _, pr := range m.prs {
		if pr.Workspace != workspace || pr.Repo != repo {
			continue
		}
		items = append(items, clonePR(pr))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return domain.PullRequestList{Workspace: workspace, Repo: repo, Count: len(items), Items: items}, nil
}

func (m *Memory) CreatePR(_ context.Context, workspace, repo string, in domain.CreatePullRequest, dryRun bool) (domain.PullRequest, error) {
	if m == nil {
		return domain.PullRequest{}, domain.Service("pr memory not configured")
	}
	workspace, repo, err := normalizePR(workspace, repo)
	if err != nil {
		return domain.PullRequest{}, err
	}
	title := strings.TrimSpace(in.Title)
	source := strings.TrimSpace(in.Source)
	if title == "" || source == "" {
		return domain.PullRequest{}, domain.Usage("create requires --title and --source").WithHint("atlas pr create --repo atlas --title '…' --source feature/branch")
	}
	target := strings.TrimSpace(in.Target)
	if target == "" {
		target = domain.DefaultTargetBranch
	}
	preview := domain.PullRequest{
		Workspace:   workspace,
		Repo:        repo,
		Title:       title,
		Description: in.Description,
		Source:      source,
		Target:      target,
		State:       "OPEN",
		Reviewers:   append([]string(nil), in.Reviewers...),
	}
	if dryRun {
		return preview, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rk := prRepoKey(workspace, repo)
	n := m.nextPR[rk]
	if n == 0 {
		n = 1
	}
	preview.ID = n
	preview.URL = domain.PRURL(workspace, repo, n)
	m.nextPR[rk] = n + 1
	m.putPRLocked(preview)
	return clonePR(preview), nil
}

func (m *Memory) EditPR(_ context.Context, workspace, repo string, id int, in domain.EditPullRequest, dryRun bool) (domain.PullRequest, error) {
	workspace, repo, err := normalizePR(workspace, repo)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if id <= 0 {
		return domain.PullRequest{}, domain.Usage("pr id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	pr, ok := m.prs[prKey(workspace, repo, id)]
	if !ok {
		return domain.PullRequest{}, domain.NotFound("pull request not found").WithHint("check --repo and --id")
	}
	if pr.State != "OPEN" {
		return domain.PullRequest{}, domain.Usage("only open pull requests can be edited")
	}
	next := clonePR(pr)
	if in.Title != nil {
		next.Title = *in.Title
	}
	if in.Description != nil {
		next.Description = *in.Description
	}
	if !dryRun {
		m.putPRLocked(next)
	}
	return next, nil
}

func (m *Memory) CommentPR(_ context.Context, workspace, repo string, id int, body string, dryRun bool) error {
	if m == nil {
		return domain.Service("pr memory not configured")
	}
	workspace, repo, err := normalizePR(workspace, repo)
	if err != nil {
		return err
	}
	if id <= 0 {
		return domain.Usage("pr id is required").WithHint("atlas pr comment --repo atlas --id 1 --body '…'")
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return domain.Usage("comment requires --body").WithHint("atlas pr comment --repo atlas --id 1 --body '…'")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	pr, ok := m.prs[prKey(workspace, repo, id)]
	if !ok {
		return domain.NotFound("pull request not found").WithHint("check --repo and --id")
	}
	if dryRun {
		return nil
	}
	pr.Comments = append(append([]string(nil), pr.Comments...), body)
	m.putPRLocked(pr)
	return nil
}

func (m *Memory) ApprovePR(_ context.Context, workspace, repo string, id int, dryRun bool) (domain.PullRequestApproval, error) {
	if m == nil {
		return domain.PullRequestApproval{}, domain.Service("pr memory not configured")
	}
	workspace, repo, err := normalizePR(workspace, repo)
	if err != nil {
		return domain.PullRequestApproval{}, err
	}
	if id <= 0 {
		return domain.PullRequestApproval{}, domain.Usage("pr id is required").WithHint("atlas pr approve --repo atlas --id 1")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := prKey(workspace, repo, id)
	pr, ok := m.prs[key]
	if !ok {
		return domain.PullRequestApproval{}, domain.NotFound("pull request not found").WithHint("check --repo and --id")
	}
	if !dryRun {
		if !m.prApprovals[key] {
			pr.Participants = append(pr.Participants, domain.PRParticipant{User: domain.PRUser{UUID: "{atlas-fake-user}", DisplayName: "Atlas fake user"}, Approved: true, Role: "REVIEWER"})
			m.putPRLocked(pr)
		}
		m.prApprovals[key] = true
	}
	return domain.PullRequestApproval{
		ID: id, Workspace: workspace, Repo: repo, URL: domain.PRURL(workspace, repo, id),
		Approved: m.prApprovals[key],
	}, nil
}

func (m *Memory) MergePR(_ context.Context, workspace, repo string, id int, dryRun bool) (domain.PullRequest, error) {
	if m == nil {
		return domain.PullRequest{}, domain.Service("pr memory not configured")
	}
	workspace, repo, err := normalizePR(workspace, repo)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if id <= 0 {
		return domain.PullRequest{}, domain.Usage("pr id is required").WithHint("atlas pr merge --repo atlas --id 1")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	pr, ok := m.prs[prKey(workspace, repo, id)]
	if !ok {
		return domain.PullRequest{}, domain.NotFound("pull request not found").WithHint("check --repo and --id")
	}
	next := clonePR(pr)
	next.State = "MERGED"
	if dryRun {
		return next, nil
	}
	m.putPRLocked(next)
	return clonePR(next), nil
}

func (m *Memory) DiffPR(_ context.Context, workspace, repo string, id int) (domain.PullRequestDiff, error) {
	if m == nil {
		return domain.PullRequestDiff{}, domain.Service("pr memory not configured")
	}
	workspace, repo, err := normalizePR(workspace, repo)
	if err != nil {
		return domain.PullRequestDiff{}, err
	}
	if id <= 0 {
		return domain.PullRequestDiff{}, domain.Usage("pr id is required").WithHint("atlas pr diff --repo atlas --id 1")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.prs[prKey(workspace, repo, id)]; !ok {
		return domain.PullRequestDiff{}, domain.NotFound("pull request not found").WithHint("check --repo and --id")
	}
	return domain.PullRequestDiff{
		ID:        id,
		Workspace: workspace,
		Repo:      repo,
		Diff:      m.prDiffs[prKey(workspace, repo, id)],
	}, nil
}

// PRCount is the seeded plus persisted PR count (tests).
func (m *Memory) PRCount() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.prs)
}

func (m *Memory) putPR(pr domain.PullRequest) {
	m.putPRLocked(pr)
}

func (m *Memory) putPRLocked(pr domain.PullRequest) {
	if pr.URL == "" && pr.ID > 0 {
		pr.URL = domain.PRURL(pr.Workspace, pr.Repo, pr.ID)
	}
	m.prs[prKey(pr.Workspace, pr.Repo, pr.ID)] = pr
}

func clonePR(pr domain.PullRequest) domain.PullRequest {
	out := pr
	if pr.Comments != nil {
		out.Comments = append([]string(nil), pr.Comments...)
	}
	if pr.Reviewers != nil {
		out.Reviewers = append([]string(nil), pr.Reviewers...)
	}
	if pr.Author != nil {
		author := *pr.Author
		out.Author = &author
	}
	if pr.Participants != nil {
		out.Participants = append([]domain.PRParticipant(nil), pr.Participants...)
	}
	return out
}

func normalizePR(workspace, repo string) (string, string, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		workspace = domain.DefaultWorkspace()
	}
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", "", domain.Usage("repo is required").WithHint("atlas pr get --repo atlas --id 1")
	}
	return workspace, repo, nil
}
