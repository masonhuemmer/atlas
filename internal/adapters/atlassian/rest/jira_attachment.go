package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func (j Jira) Attachments(ctx context.Context, hostname, key string) ([]domain.JiraAttachment, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return nil, err
	}
	u := joinURL(j.origin(hostname), "/rest/api/3/issue/"+q(strings.ToUpper(strings.TrimSpace(key)))+"?fields=attachment")
	code, body, err := j.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, MapStatus(code)
	}
	var response struct {
		Fields struct {
			Attachments []struct {
				ID       string `json:"id"`
				Filename string `json:"filename"`
				Size     int64  `json:"size"`
				MIMEType string `json:"mimeType"`
				Created  string `json:"created"`
			} `json:"attachment"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, domain.Service("Jira attachments returned invalid JSON")
	}
	items := make([]domain.JiraAttachment, 0, len(response.Fields.Attachments))
	for _, a := range response.Fields.Attachments {
		items = append(items, domain.JiraAttachment{ID: a.ID, Filename: a.Filename, Size: a.Size, MIMEType: a.MIMEType, Created: a.Created})
	}
	return items, nil
}

func (j Jira) DownloadAttachment(ctx context.Context, hostname, id string, dst io.Writer) (int64, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return 0, err
	}
	u := joinURL(j.origin(hostname), "/rest/api/3/attachment/content/"+q(id)+"?redirect=false")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, domain.Usage(err.Error())
	}
	req.SetBasicAuth(cred.Email, cred.Token)
	req.Header.Set("Accept", "application/octet-stream")
	client := *j.httpc()
	client.Timeout = 10 * time.Minute
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(req)
	if err != nil {
		return 0, domain.Service(err.Error())
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		if res.StatusCode >= 300 && res.StatusCode < 400 {
			return 0, domain.Service("Jira attachment download redirected unexpectedly")
		}
		if res.StatusCode == http.StatusNotFound {
			return 0, domain.NotFound("attachment not found")
		}
		return 0, MapStatus(res.StatusCode)
	}
	n, err := io.Copy(dst, res.Body)
	if err != nil {
		return n, domain.Service(err.Error())
	}
	return n, nil
}
