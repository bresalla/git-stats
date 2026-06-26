package bitbucket

type RawAuthor struct {
	Raw  string `json:"raw"`
	User struct {
		AccountID   string `json:"account_id"`
		DisplayName string `json:"display_name"`
	} `json:"user"`
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
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	CreatedOn string    `json:"created_on"`
	UpdatedOn string    `json:"updated_on"`
	Author    RawAuthor `json:"author"`
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
