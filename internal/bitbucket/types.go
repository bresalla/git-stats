package bitbucket

type RawAuthor struct {
	Raw  string `json:"raw"`
	User struct {
		AccountID   string `json:"account_id"`
		DisplayName string `json:"display_name"`
	} `json:"user"`
}

// RawUser represents a Bitbucket user/account object as returned directly
// (not wrapped in a "user" sub-object) by endpoints such as pull requests,
// where the actor is always an authenticated Bitbucket account rather than
// an arbitrary git commit author string.
type RawUser struct {
	AccountID   string `json:"account_id"`
	DisplayName string `json:"display_name"`
}

type RawRepository struct {
	Slug string `json:"slug"`
}

type repositoriesPage struct {
	Values []RawRepository `json:"values"`
	Next   string          `json:"next"`
}

type RawCommit struct {
	Hash    string    `json:"hash"`
	Message string    `json:"message"`
	Date    string    `json:"date"`
	Author  RawAuthor `json:"author"`
}

type commitsPage struct {
	Values []RawCommit `json:"values"`
	Next   string      `json:"next"`
}

type RawDiffstatEntry struct {
	Status       string `json:"status"`
	LinesAdded   int    `json:"lines_added"`
	LinesRemoved int    `json:"lines_removed"`
	New          *struct {
		Path string `json:"path"`
	} `json:"new"`
	Old *struct {
		Path string `json:"path"`
	} `json:"old"`
}

type diffstatPage struct {
	Values []RawDiffstatEntry `json:"values"`
	Next   string              `json:"next"`
}

type RawPullRequest struct {
	ID        int     `json:"id"`
	Title     string  `json:"title"`
	State     string  `json:"state"`
	CreatedOn string  `json:"created_on"`
	UpdatedOn string  `json:"updated_on"`
	Author    RawUser `json:"author"`
}

type pullRequestsPage struct {
	Values []RawPullRequest `json:"values"`
	Next   string           `json:"next"`
}

type RawActivity struct {
	Approval *struct {
		Date string    `json:"date"`
		User RawAuthor `json:"user"`
	} `json:"approval"`
	Comment *struct {
		CreatedOn string    `json:"created_on"`
		User      RawAuthor `json:"user"`
	} `json:"comment"`
}

type activityPage struct {
	Values []RawActivity `json:"values"`
	Next   string        `json:"next"`
}
