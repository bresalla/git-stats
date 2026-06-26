package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	BaseURL     string
	Username    string
	AppPassword string
	HTTPClient  *http.Client
}

func NewClient(username, appPassword string) *Client {
	return &Client{
		BaseURL:     "https://api.bitbucket.org/2.0",
		Username:    username,
		AppPassword: appPassword,
		HTTPClient:  http.DefaultClient,
	}
}

func (c *Client) get(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.Username, c.AppPassword)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bitbucket API request to %s failed: status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) ListCommits(ctx context.Context, workspace, repoSlug string, since time.Time) ([]RawCommit, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/commits", c.BaseURL, workspace, repoSlug)

	var all []RawCommit
	for url != "" {
		var page commitsPage
		if err := c.get(ctx, url, &page); err != nil {
			return nil, fmt.Errorf("listing commits for %s/%s: %w", workspace, repoSlug, err)
		}
		all = append(all, page.Values...)
		url = page.Next
	}
	return all, nil
}

func (c *Client) GetDiffstat(ctx context.Context, workspace, repoSlug, commitHash string) ([]RawDiffstatEntry, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/diffstat/%s", c.BaseURL, workspace, repoSlug, commitHash)

	var all []RawDiffstatEntry
	for url != "" {
		var page diffstatPage
		if err := c.get(ctx, url, &page); err != nil {
			return nil, fmt.Errorf("getting diffstat for %s/%s@%s: %w", workspace, repoSlug, commitHash, err)
		}
		all = append(all, page.Values...)
		url = page.Next
	}
	return all, nil
}

func (c *Client) ListPullRequests(ctx context.Context, workspace, repoSlug string) ([]RawPullRequest, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests?state=MERGED&state=OPEN&state=DECLINED&state=SUPERSEDED", c.BaseURL, workspace, repoSlug)

	var all []RawPullRequest
	for url != "" {
		var page pullRequestsPage
		if err := c.get(ctx, url, &page); err != nil {
			return nil, fmt.Errorf("listing pull requests for %s/%s: %w", workspace, repoSlug, err)
		}
		all = append(all, page.Values...)
		url = page.Next
	}
	return all, nil
}

func (c *Client) ListActivity(ctx context.Context, workspace, repoSlug string, pullRequestID int) ([]RawActivity, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/activity", c.BaseURL, workspace, repoSlug, pullRequestID)

	var all []RawActivity
	for url != "" {
		var page activityPage
		if err := c.get(ctx, url, &page); err != nil {
			return nil, fmt.Errorf("listing activity for %s/%s PR %d: %w", workspace, repoSlug, pullRequestID, err)
		}
		all = append(all, page.Values...)
		url = page.Next
	}
	return all, nil
}
