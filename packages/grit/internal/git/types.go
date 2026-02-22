package git

type BranchStatus struct {
	OID      string `json:"oid"`
	Head     string `json:"head"`
	Upstream string `json:"upstream,omitempty"`
	Ahead    int    `json:"ahead,omitempty"`
	Behind   int    `json:"behind,omitempty"`
}

type StatusEntry struct {
	State    string `json:"state"`
	Path     string `json:"path"`
	OrigPath string `json:"orig_path,omitempty"`
}

type StatusResult struct {
	Branch  BranchStatus  `json:"branch"`
	Entries []StatusEntry `json:"entries"`
}

type DiffStat struct {
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Path      string `json:"path"`
	Binary    bool   `json:"binary,omitempty"`
}

type DiffSummary struct {
	TotalFiles     int `json:"total_files"`
	TotalAdditions int `json:"total_additions"`
	TotalDeletions int `json:"total_deletions"`
}

type DiffResult struct {
	Stats           []DiffStat  `json:"stats"`
	Summary         DiffSummary `json:"summary"`
	Patch           string      `json:"patch,omitempty"`
	Truncated       bool        `json:"truncated,omitempty"`
	TruncatedAtLine int         `json:"truncated_at_line,omitempty"`
}

type LogEntry struct {
	Hash        string `json:"hash"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
	AuthorDate  string `json:"author_date"`
	Subject     string `json:"subject"`
	Body        string `json:"body,omitempty"`
}

type ShowResult struct {
	Hash            string     `json:"hash"`
	AuthorName      string     `json:"author_name"`
	AuthorEmail     string     `json:"author_email"`
	AuthorDate      string     `json:"author_date"`
	Subject         string     `json:"subject"`
	Body            string     `json:"body,omitempty"`
	Stats           []DiffStat `json:"stats"`
	Patch           string     `json:"patch,omitempty"`
	Truncated       bool       `json:"truncated,omitempty"`
	TruncatedAtLine int        `json:"truncated_at_line,omitempty"`
}

type BlameLine struct {
	Hash        string `json:"hash"`
	OrigLine    int    `json:"orig_line"`
	FinalLine   int    `json:"final_line"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
	AuthorDate  string `json:"author_date"`
	Summary     string `json:"summary"`
	Content     string `json:"content"`
}

type BranchEntry struct {
	Name      string `json:"name"`
	Hash      string `json:"hash"`
	Subject   string `json:"subject"`
	IsCurrent bool   `json:"is_current"`
	Upstream  string `json:"upstream,omitempty"`
	Track     string `json:"track,omitempty"`
}

type RemoteEntry struct {
	Name     string `json:"name"`
	FetchURL string `json:"fetch_url"`
	PushURL  string `json:"push_url"`
}

type RevParseResult struct {
	Resolved string `json:"resolved"`
	Ref      string `json:"ref"`
}

type CommitResult struct {
	Status  string `json:"status"`
	Branch  string `json:"branch"`
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
}

type PullResult struct {
	Status  string `json:"status"`
	Summary string `json:"summary,omitempty"`
}

type MutationResult struct {
	Status      string   `json:"status"`
	Paths       []string `json:"paths,omitempty"`
	Name        string   `json:"name,omitempty"`
	Ref         string   `json:"ref,omitempty"`
	StartPoint  string   `json:"start_point,omitempty"`
	Create      bool     `json:"create,omitempty"`
	Remote      string   `json:"remote,omitempty"`
	Branch      string   `json:"branch,omitempty"`
	SetUpstream bool     `json:"set_upstream,omitempty"`
	Force       bool     `json:"force,omitempty"`
	All         bool     `json:"all,omitempty"`
	Prune       bool     `json:"prune,omitempty"`
}

type RebaseResult struct {
	Status      string   `json:"status"`
	Branch      string   `json:"branch,omitempty"`
	Upstream    string   `json:"upstream,omitempty"`
	Conflicts   []string `json:"conflicts,omitempty"`
	CurrentStep string   `json:"current_step,omitempty"`
	Summary     string   `json:"summary,omitempty"`
}
