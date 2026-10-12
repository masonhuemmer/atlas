package rest

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// Bitbucket is the live Cloud REST client (https://api.bitbucket.org/2.0).
// Auth uses the keyring credential stored for the requested workspace.
type Bitbucket struct {
	*Client
}

func (b Bitbucket) Get(ctx context.Context, workspace, repo string, id int) (domain.PullRequest, error) {
	cred, err := b.workspaceCred(workspace)
	if err != nil {
		return domain.PullRequest{}, err
	}
	u := joinURL(b.bitbucketOrigin(), "/2.0/repositories/"+q(workspace)+"/"+q(repo)+"/pullrequests/"+strconv.Itoa(id))
	code, body, err := b.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if code != http.StatusOK {
		return domain.PullRequest{}, MapBitbucketStatus(code)
	}
	return prFromREST(workspace, repo, mustJSON(body)), nil
}

func (b Bitbucket) Comments(ctx context.Context, workspace, repo string, id, page, limit int) (domain.PullRequestCommentPage, error) {
	cred, err := b.workspaceCred(workspace)
	if err != nil {
		return domain.PullRequestCommentPage{}, err
	}
	u := joinURL(b.bitbucketOrigin(), "/2.0/repositories/"+q(workspace)+"/"+q(repo)+"/pullrequests/"+strconv.Itoa(id)+"/comments?page="+strconv.Itoa(page)+"&pagelen="+strconv.Itoa(limit))
	code, body, err := b.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.PullRequestCommentPage{}, err
	}
	if code != http.StatusOK {
		return domain.PullRequestCommentPage{}, MapBitbucketStatus(code)
	}
	m := mustJSON(body)
	result := domain.PullRequestCommentPage{ID: id, Workspace: workspace, Repo: repo, Comments: []domain.PullRequestComment{}}
	for _, raw := range asList(m["values"]) {
		comment := asMap(raw)
		commentID := 0
		if n, ok := comment["id"].(float64); ok {
			commentID = int(n)
		}
		result.Comments = append(result.Comments, domain.PullRequestComment{
			ID: commentID, Body: str(asMap(comment["content"]), "raw"), Author: prUserFromREST(asMap(comment["user"])),
		})
	}
	if str(m, "next") != "" {
		result.NextPage = page + 1
	}
	return result, nil
}

func (b Bitbucket) List(ctx context.Context, workspace, repo string) (domain.PullRequestList, error) {
	cred, err := b.workspaceCred(workspace)
	if err != nil {
		return domain.PullRequestList{}, err
	}
	u := joinURL(b.bitbucketOrigin(), "/2.0/repositories/"+q(workspace)+"/"+q(repo)+"/pullrequests?state=OPEN")
	code, body, err := b.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.PullRequestList{}, err
	}
	if code != http.StatusOK {
		return domain.PullRequestList{}, MapBitbucketStatus(code)
	}
	var items []domain.PullRequest
	for _, raw := range asList(mustJSON(body)["values"]) {
		items = append(items, prFromREST(workspace, repo, asMap(raw)))
	}
	if items == nil {
		items = []domain.PullRequest{}
	}
	return domain.PullRequestList{Workspace: workspace, Repo: repo, Count: len(items), Items: items}, nil
}

func (b Bitbucket) Create(ctx context.Context, workspace, repo string, in domain.CreatePullRequest, dryRun bool) (domain.PullRequest, error) {
	target := in.Target
	if target == "" {
		target = domain.DefaultTargetBranch
	}
	preview := domain.PullRequest{
		Workspace: workspace, Repo: repo, Title: in.Title, Description: in.Description,
		Source: in.Source, Target: target, State: "OPEN", Reviewers: in.Reviewers,
	}
	if dryRun {
		return preview, nil
	}
	cred, err := b.workspaceCred(workspace)
	if err != nil {
		return domain.PullRequest{}, err
	}
	payload := map[string]any{
		"title": in.Title,
		"source": map[string]any{
			"branch": map[string]any{"name": in.Source},
		},
		"destination": map[string]any{
			"branch": map[string]any{"name": target},
		},
	}
	if in.Description != "" {
		payload["description"] = in.Description
	}
	if len(in.Reviewers) > 0 {
		var revs []any
		for _, r := range in.Reviewers {
			revs = append(revs, map[string]any{"uuid": r})
		}
		payload["reviewers"] = revs
	}
	u := joinURL(b.bitbucketOrigin(), "/2.0/repositories/"+q(workspace)+"/"+q(repo)+"/pullrequests")
	code, body, err := b.doJSON(ctx, http.MethodPost, u, cred, nil, payload)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if code != http.StatusCreated && code != http.StatusOK {
		return domain.PullRequest{}, MapBitbucketStatus(code)
	}
	return prFromREST(workspace, repo, mustJSON(body)), nil
}

func (b Bitbucket) Edit(ctx context.Context, workspace, repo string, id int, in domain.EditPullRequest, dryRun bool) (domain.PullRequest, error) {
	current, err := b.Get(ctx, workspace, repo, id)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if current.State != "OPEN" {
		return domain.PullRequest{}, domain.Usage("only open pull requests can be edited")
	}
	if in.Title != nil {
		current.Title = *in.Title
	}
	if in.Description != nil {
		current.Description = *in.Description
	}
	if dryRun {
		return current, nil
	}
	cred, err := b.workspaceCred(workspace)
	if err != nil {
		return domain.PullRequest{}, err
	}
	u := joinURL(b.bitbucketOrigin(), "/2.0/repositories/"+q(workspace)+"/"+q(repo)+"/pullrequests/"+strconv.Itoa(id))
	code, body, err := b.doJSON(ctx, http.MethodPut, u, cred, nil, map[string]any{
		"title": current.Title, "description": current.Description,
	})
	if err != nil {
		return domain.PullRequest{}, err
	}
	if code != http.StatusOK {
		return domain.PullRequest{}, MapBitbucketStatus(code)
	}
	return prFromREST(workspace, repo, mustJSON(body)), nil
}

func (b Bitbucket) Comment(ctx context.Context, workspace, repo string, id int, body string, dryRun bool) error {
	if dryRun {
		return nil
	}
	cred, err := b.workspaceCred(workspace)
	if err != nil {
		return err
	}
	u := joinURL(b.bitbucketOrigin(), "/2.0/repositories/"+q(workspace)+"/"+q(repo)+"/pullrequests/"+strconv.Itoa(id)+"/comments")
	code, _, err := b.doJSON(ctx, http.MethodPost, u, cred, nil, map[string]any{"content": map[string]any{"raw": body}})
	if err != nil {
		return err
	}
	if code != http.StatusCreated && code != http.StatusOK {
		return MapBitbucketStatus(code)
	}
	return nil
}

func (b Bitbucket) Approve(ctx context.Context, workspace, repo string, id int, dryRun bool) (domain.PullRequestApproval, error) {
	approval := domain.PullRequestApproval{ID: id, Workspace: workspace, Repo: repo, URL: domain.PRURL(workspace, repo, id)}
	if dryRun {
		return approval, nil
	}
	cred, err := b.workspaceCred(workspace)
	if err != nil {
		return domain.PullRequestApproval{}, err
	}
	u := joinURL(b.bitbucketOrigin(), "/2.0/repositories/"+q(workspace)+"/"+q(repo)+"/pullrequests/"+strconv.Itoa(id)+"/approve")
	code, body, err := b.doJSON(ctx, http.MethodPost, u, cred, nil, nil)
	if err != nil {
		return domain.PullRequestApproval{}, err
	}
	if code != http.StatusOK {
		return domain.PullRequestApproval{}, MapBitbucketStatus(code)
	}
	if approved, _ := mustJSON(body)["approved"].(bool); !approved {
		return domain.PullRequestApproval{}, domain.Service("Bitbucket did not confirm pull request approval")
	}
	approval.Approved = true
	return approval, nil
}

func (b Bitbucket) Merge(ctx context.Context, workspace, repo string, id int, dryRun bool) (domain.PullRequest, error) {
	if dryRun {
		return b.Get(ctx, workspace, repo, id)
	}
	cred, err := b.workspaceCred(workspace)
	if err != nil {
		return domain.PullRequest{}, err
	}
	u := joinURL(b.bitbucketOrigin(), "/2.0/repositories/"+q(workspace)+"/"+q(repo)+"/pullrequests/"+strconv.Itoa(id)+"/merge")
	code, body, err := b.doJSON(ctx, http.MethodPost, u, cred, nil, map[string]any{})
	if err != nil {
		return domain.PullRequest{}, err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return domain.PullRequest{}, MapBitbucketStatus(code)
	}
	return prFromREST(workspace, repo, mustJSON(body)), nil
}

func (b Bitbucket) Diff(ctx context.Context, workspace, repo string, id int) (domain.PullRequestDiff, error) {
	cred, err := b.workspaceCred(workspace)
	if err != nil {
		return domain.PullRequestDiff{}, err
	}
	u := joinURL(b.bitbucketOrigin(), "/2.0/repositories/"+q(workspace)+"/"+q(repo)+"/pullrequests/"+strconv.Itoa(id)+"/diff")
	code, body, err := b.doBytes(ctx, http.MethodGet, u, cred, nil)
	if err != nil {
		return domain.PullRequestDiff{}, err
	}
	if code != http.StatusOK {
		return domain.PullRequestDiff{}, MapBitbucketStatus(code)
	}
	return domain.PullRequestDiff{ID: id, Workspace: workspace, Repo: repo, Diff: string(body)}, nil
}

func prFromREST(workspace, repo string, m map[string]any) domain.PullRequest {
	id := 0
	if n, ok := m["id"].(float64); ok {
		id = int(n)
	}
	src := str(asMap(asMap(m["source"])["branch"]), "name")
	dst := str(asMap(asMap(m["destination"])["branch"]), "name")
	state := strings.ToUpper(str(m, "state"))
	var reviewers []string
	for _, raw := range asList(m["reviewers"]) {
		if user := prUserFromREST(asMap(raw)); user != nil {
			reviewers = append(reviewers, user.DisplayName)
		}
	}
	var participants []domain.PRParticipant
	for _, raw := range asList(m["participants"]) {
		participant := asMap(raw)
		user := prUserFromREST(asMap(participant["user"]))
		if user == nil {
			continue
		}
		approved, _ := participant["approved"].(bool)
		participants = append(participants, domain.PRParticipant{User: *user, Approved: approved, Role: str(participant, "role")})
	}
	return domain.PullRequest{
		ID: id, Workspace: workspace, Repo: repo, Title: str(m, "title"),
		Description: str(m, "description"), Source: src, Target: dst, State: state,
		URL: domain.PRURL(workspace, repo, id), Reviewers: reviewers,
		Author: prUserFromREST(asMap(m["author"])), Participants: participants,
	}
}

func prUserFromREST(m map[string]any) *domain.PRUser {
	if m == nil {
		return nil
	}
	return &domain.PRUser{UUID: str(m, "uuid"), AccountID: str(m, "account_id"), DisplayName: str(m, "display_name")}
}

func MapBitbucketStatus(code int) error {
	return classify(code, "bitbucket", "pull request not found")
}
