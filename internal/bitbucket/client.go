package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const maxRetries = 5

type Client struct {
	BaseURL    string
	Email      string
	APIToken   string
	HTTPClient *http.Client
}

func NewClient(email, apiToken string) *Client {
	return &Client{
		BaseURL:    "https://api.bitbucket.org/2.0",
		Email:      email,
		APIToken:   apiToken,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) get(ctx context.Context, url string, out any) error {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		req.SetBasicAuth(c.Email, c.APIToken)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries {
			wait := retryAfter(resp.Header.Get("Retry-After"))
			resp.Body.Close()
			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bitbucket API request to %s failed: status %d", url, resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(out)
	}
}

func retryAfter(header string) time.Duration {
	if secs, err := strconv.Atoi(header); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 5 * time.Second
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

func (c *Client) ListRepositories(ctx context.Context, workspace string) ([]string, error) {
	url := fmt.Sprintf("%s/repositories/%s", c.BaseURL, workspace)

	var slugs []string
	for url != "" {
		var page repositoriesPage
		if err := c.get(ctx, url, &page); err != nil {
			return nil, fmt.Errorf("listing repositories for %s: %w", workspace, err)
		}
		for _, repo := range page.Values {
			slugs = append(slugs, repo.Slug)
		}
		url = page.Next
	}
	return slugs, nil
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
