package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type GitCloneConfig struct {
	BaseDir string
}

func NewGitCloneTool(ctx context.Context, config *GitCloneConfig) (tool.BaseTool, error) {
	if config == nil {
		config = &GitCloneConfig{}
	}
	if config.BaseDir == "" {
		config.BaseDir = "repos"
	}

	// Make BaseDir an absolute path to prevent ambiguity.
	absBaseDir, err := filepath.Abs(config.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("could not get absolute path for base dir: %w", err)
	}
	config.BaseDir = absBaseDir

	return utils.InferTool(
		"gitclone",
		"Clone or pull a Git repository into a secure, local directory. CRITICAL: The response returns a 'path' field - you MUST use this EXACT path when calling other file tools. Use action='clone' for new repos, action='pull' to update existing ones.",
		func(ctx context.Context, req *GitCloneRequest) (*GitCloneResponse, error) {
			return invokeGitClone(ctx, req, config)
		},
	)
}

type GitCloneAction string

const (
	GitCloneActionClone GitCloneAction = "clone"
	GitCloneActionPull  GitCloneAction = "pull"
)

type GitCloneRequest struct {
	Url    string         `json:"url" jsonschema:"description=The URL of the repository to clone (HTTPS or SSH format)."`
	Action GitCloneAction `json:"action" jsonschema:"description=The action to perform: 'clone' or 'pull'."`
}

type GitCloneResponse struct {
	Message   string `json:"message" jsonschema:"description=Success message describing the result."`
	Path      string `json:"path,omitempty" jsonschema:"description=The full, safe local path to the repository. Use this in subsequent tool calls."`
	NextSteps string `json:"next_steps,omitempty" jsonschema:"description=Suggested next actions to explore the repository."`
	Error     string `json:"error,omitempty" jsonschema:"description=Error message if the operation failed."`
}

// gitURLRegex is a robust regex to parse different Git URL formats.
var gitURLRegex = regexp.MustCompile(`^(?:(?:https?|git)://|git@)(?P<host>[^:/]+)[:/](?P<org>[^/]+)/(?P<repo>[^/]+?)(?:\.git)?$`)

type parsedURL struct {
	Host, Org, Repo string
}

func parseAndSanitizeURL(url string) (*parsedURL, error) {
	if !gitURLRegex.MatchString(url) {
		return nil, fmt.Errorf("invalid or unsupported git URL format")
	}
	matches := gitURLRegex.FindStringSubmatch(url)
	names := gitURLRegex.SubexpNames()

	result := &parsedURL{}
	for i, name := range names {
		if i != 0 && name != "" {
			// **CRITICAL SECURITY STEP**: Sanitize components to prevent path traversal.
			sanitizedValue := strings.ReplaceAll(matches[i], ".", "_")   // Replace dots to be safe
			sanitizedValue = strings.ReplaceAll(sanitizedValue, "/", "") // Should not happen with regex, but belt-and-suspenders.

			switch name {
			case "host":
				result.Host = sanitizedValue
			case "org":
				result.Org = sanitizedValue
			case "repo":
				result.Repo = sanitizedValue
			}
		}
	}

	if result.Org == "" || result.Repo == "" {
		return nil, fmt.Errorf("could not extract organization and repository from URL")
	}
	return result, nil
}

func invokeGitClone(ctx context.Context, req *GitCloneRequest, config *GitCloneConfig) (*GitCloneResponse, error) {
	if req.Url == "" {
		return &GitCloneResponse{Error: "URL cannot be empty"}, nil
	}
	if req.Action == "" {
		return &GitCloneResponse{Error: "action must be 'clone' or 'pull'"}, nil
	}

	parsed, err := parseAndSanitizeURL(req.Url)
	if err != nil {
		return &GitCloneResponse{Error: err.Error()}, nil
	}

	// Construct a safe, predictable path.
	repoPath := filepath.Join(config.BaseDir, parsed.Host, parsed.Org, parsed.Repo)

	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return &GitCloneResponse{Error: fmt.Sprintf("failed to create parent directory: %v", err)}, nil
	}

	switch req.Action {
	case GitCloneActionClone:
		if _, err := os.Stat(repoPath); err == nil {
			return &GitCloneResponse{
				Error: fmt.Sprintf("repository already exists at '%s'. Did you mean to use action='pull'?", repoPath),
				Path:  repoPath,
			}, nil
		}

		_, err := git.PlainCloneContext(ctx, repoPath, false, &git.CloneOptions{
			URL:           req.Url, // Use original URL for cloning
			Depth:         1,       // Shallow clone for speed and space
			SingleBranch:  true,
			ReferenceName: plumbing.HEAD,
		})
		if err != nil {
			return &GitCloneResponse{Error: fmt.Sprintf("clone failed: %v", err)}, nil
		}

	case GitCloneActionPull:
		repo, err := git.PlainOpen(repoPath)
		if err != nil {
			if err == git.ErrRepositoryNotExists {
				return &GitCloneResponse{Error: fmt.Sprintf("repository does not exist at '%s'. Did you mean to use action='clone'?", repoPath)}, nil
			}
			return &GitCloneResponse{Error: fmt.Sprintf("failed to open repository: %v", err)}, nil
		}

		w, err := repo.Worktree()
		if err != nil {
			return &GitCloneResponse{Error: fmt.Sprintf("failed to get worktree: %v", err)}, nil
		}

		// **ROBUSTNESS CHECK**: Ensure worktree is clean before pulling.
		status, err := w.Status()
		if err != nil {
			return &GitCloneResponse{Error: fmt.Sprintf("failed to get worktree status: %v", err)}, nil
		}
		if !status.IsClean() {
			return &GitCloneResponse{Error: "cannot pull: repository has uncommitted changes"}, nil
		}

		err = w.PullContext(ctx, &git.PullOptions{RemoteName: "origin"})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return &GitCloneResponse{Error: fmt.Sprintf("pull failed: %v", err)}, nil
		}

	default:
		return &GitCloneResponse{Error: fmt.Sprintf("invalid action '%s', use 'clone' or 'pull'", req.Action)}, nil
	}

	return &GitCloneResponse{
		Message: fmt.Sprintf("Successfully %sd repository to '%s'", req.Action, repoPath),
		Path:    repoPath,
		NextSteps: fmt.Sprintf("IMPORTANT: Use the EXACT path '%s' with all file tools. Examples:\n- search_files(path='%s', pattern='**/*.go')\n- read_file(path='%s/README.md')",
			repoPath, repoPath, repoPath),
	}, nil
}
