package atlassian

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/masonhuemmer/atlas/internal/domain"
)

const seedPageID = "100"

type seedFile struct {
	Issues []seedIssue `json:"issues"`
	Users  []seedUser  `json:"users"`
	Pages  []seedPage  `json:"pages"`
	PRs    []seedPR    `json:"pull_requests"`
	Desks  []seedDesk  `json:"desks"`
	Types  []seedType  `json:"types"`
	Reqs   []seedReq   `json:"requests"`
}

type seedUser struct {
	Alias       string `json:"alias"`
	AccountID   string `json:"account_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Active      bool   `json:"active"`
}

type seedIssue struct {
	Key         string   `json:"key"`
	Alias       string   `json:"alias"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	IssueType   string   `json:"issuetype"`
	Priority    string   `json:"priority"`
	Labels      []string `json:"labels"`
	Assignee    string   `json:"assignee"`
	Reporter    string   `json:"reporter"`
	Created     string   `json:"created"`
	Updated     string   `json:"updated"`
	Project     string   `json:"project"`
	Transitions []string `json:"transitions"`
}

type seedPage struct {
	ID    string `json:"id"`
	Alias string `json:"alias"`
	Space string `json:"space"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type seedPR struct {
	ID          int    `json:"id"`
	Repo        string `json:"repo"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Diff        string `json:"diff"`
}

type seedDesk struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type seedType struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	DeskID string `json:"desk_id"`
}

type seedReq struct {
	Key            string   `json:"key"`
	Alias          string   `json:"alias"`
	DeskID         string   `json:"desk_id"`
	TypeID         string   `json:"type_id"`
	Summary        string   `json:"summary"`
	Description    string   `json:"description"`
	Status         string   `json:"status"`
	StatusCategory string   `json:"status_category"`
	Transitions    []string `json:"transitions"`
}

// Seed returns in-memory issues, pages, PRs, and JSM desks for ATLAS_FAKE and tests.
// Hostnames and workspace come from the active catalog; keys come from testdata/seed.json.
func Seed() *Memory {
	m := &Memory{
		issues:         map[string]domain.Issue{},
		transitions:    map[string][]string{},
		next:           map[string]int{},
		pages:          map[string]domain.Page{},
		spaces:         map[string]string{},
		nextPage:       101,
		prs:            map[string]domain.PullRequest{},
		prDiffs:        map[string]string{},
		nextPR:         map[string]int{},
		types:          map[string][]domain.RequestType{},
		requests:       map[string]domain.CustomerRequest{},
		jsmTransitions: map[string][]string{},
		nextReq:        map[string]int{},
	}
	var f seedFile
	b, err := os.ReadFile(seedDataPath())
	if err != nil {
		panic("atlas seed: " + err.Error())
	}
	if err := json.Unmarshal(b, &f); err != nil {
		panic("atlas seed: " + err.Error())
	}
	for _, iss := range f.Issues {
		host := hostForAlias(iss.Alias)
		item := domain.Issue{
			Key: iss.Key, Site: host, BrowseURL: domain.BrowseURL(host, iss.Key),
			Summary: iss.Summary, Description: iss.Description,
			Status: iss.Status, IssueType: iss.IssueType, Priority: iss.Priority,
			Labels: iss.Labels, Assignee: iss.Assignee, Reporter: iss.Reporter,
			Created: iss.Created, Updated: iss.Updated, Project: iss.Project,
		}
		m.put(item)
		trans := iss.Transitions
		if len(trans) == 0 {
			trans = []string{"Done"}
		}
		m.transitions[memKey(item.Site, item.Key)] = trans
		if n := issueNumber(iss.Key); n+1 > m.next[iss.Project] {
			m.next[iss.Project] = n + 1
		}
	}
	for _, user := range f.Users {
		m.users = append(m.users, domain.JiraUser{
			Site: hostForAlias(user.Alias), AccountID: user.AccountID,
			DisplayName: user.DisplayName, Email: user.Email, Active: user.Active,
		})
	}
	for _, p := range f.Pages {
		host := hostForAlias(p.Alias)
		m.spaces[p.Space] = p.Space + "-space-id"
		m.putPage(domain.Page{
			ID:            p.ID,
			Site:          host,
			Space:         p.Space,
			Title:         p.Title,
			Body:          p.Body,
			ContentFormat: domain.StorageBodyFormat,
			Status:        "current",
			URL:           domain.WikiPageURL(host, p.Space, p.ID),
			Version:       1,
		})
		if n := issueNumber(p.ID); n+1 > m.nextPage {
			m.nextPage = n + 1
		}
	}
	ws := domain.DefaultWorkspace()
	for _, pr := range f.PRs {
		item := domain.PullRequest{
			ID:          pr.ID,
			Workspace:   ws,
			Repo:        pr.Repo,
			Title:       pr.Title,
			Description: pr.Description,
			Source:      pr.Source,
			Target:      domain.DefaultTargetBranch,
			State:       "OPEN",
			URL:         domain.PRURL(ws, pr.Repo, pr.ID),
		}
		m.putPR(item)
		m.prDiffs[prKey(item.Workspace, item.Repo, item.ID)] = pr.Diff
		if pr.ID+1 > m.nextPR[prRepoKey(item.Workspace, item.Repo)] {
			m.nextPR[prRepoKey(item.Workspace, item.Repo)] = pr.ID + 1
		}
	}
	for _, d := range f.Desks {
		m.desks = append(m.desks, domain.ServiceDesk{ID: d.ID, Key: d.Key, Name: d.Name})
		if m.nextReq[d.Key] == 0 {
			m.nextReq[d.Key] = 1
		}
	}
	for _, typ := range f.Types {
		m.types[typ.DeskID] = append(m.types[typ.DeskID], domain.RequestType{ID: typ.ID, Name: typ.Name, DeskID: typ.DeskID})
	}
	for _, req := range f.Reqs {
		host := hostForAlias(req.Alias)
		item := domain.CustomerRequest{
			Key:            req.Key,
			Site:           host,
			DeskID:         req.DeskID,
			TypeID:         req.TypeID,
			Summary:        req.Summary,
			Description:    req.Description,
			Status:         req.Status,
			StatusCategory: req.StatusCategory,
			PortalURL:      domain.PortalURL(host, req.DeskID, req.Key),
		}
		m.putRequest(item)
		m.jsmTransitions[memKey(host, req.Key)] = req.Transitions
		if n := issueNumber(req.Key); n+1 > m.nextReq[projectOf(req.Key)] {
			m.nextReq[projectOf(req.Key)] = n + 1
		}
	}
	return m
}

func seedDataPath() string {
	var candidates []string
	if _, file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(file), "testdata", "seed.json"))
	}
	candidates = append(candidates,
		filepath.Join("testdata", "seed.json"),
		filepath.Join("internal", "adapters", "atlassian", "testdata", "seed.json"),
	)
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join("testdata", "seed.json")
}

func hostForAlias(alias string) string {
	s, err := domain.Lookup(alias)
	if err != nil {
		return alias
	}
	return s.Hostname
}

func projectOf(key string) string {
	p, ok := domain.ProjectFromIssue(key)
	if !ok {
		return key
	}
	return p
}

func (m *Memory) put(iss domain.Issue) {
	m.putLocked(iss)
}

func issueNumber(key string) int {
	i := len(key) - 1
	n := 0
	mult := 1
	for i >= 0 && key[i] >= '0' && key[i] <= '9' {
		n += int(key[i]-'0') * mult
		mult *= 10
		i--
	}
	return n
}

func memKey(hostname, key string) string {
	return hostname + "\x00" + key
}

func prRepoKey(workspace, repo string) string {
	return workspace + "\x00" + repo
}

func prKey(workspace, repo string, id int) string {
	return workspace + "\x00" + repo + "\x00" + strconv.Itoa(id)
}
