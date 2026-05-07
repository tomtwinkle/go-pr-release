package release

type Config struct {
	WorkDir               string
	RemoteName            string
	Repository            Repository
	Token                 string
	Title                 string
	ProductionBranch      string
	StagingBranch         string
	TemplatePath          string
	Labels                []string
	ExtraReviewers        []string
	Mention               string
	AssignPRAuthor        bool
	RequestPRAuthorReview bool
	DryRun                bool
	JSON                  bool
	NoFetch               bool
	Squashed              bool
	OverwriteDescription  bool
	Verbose               bool
	InsecureSkipTLSVerify bool
}
