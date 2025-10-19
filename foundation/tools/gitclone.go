package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type GitCloneConfig struct {
	BaseDir string
}

type GitCloneImpl struct {
	config *GitCloneConfig
}

func NewGitCloneTool(ctx context.Context, config *GitCloneConfig) (tool.BaseTool, error) {
	if config == nil {
		config = &GitCloneConfig{
			BaseDir: "repos",
		}
	}
	if config.BaseDir == "" {
		return nil, fmt.Errorf("base dir cannot be empty")
	}

	impl := &GitCloneImpl{config: config}

	return utils.InferTool(
		"gitclone",
		"Clone or pull a Git repository to the 'repos' directory. CRITICAL: The response returns a 'path' field - you MUST use this EXACT path when calling search_files, read_file, or any file operation. Example: if path='repos/cloudwego/eino', then use search_files(path='repos/cloudwego/eino'). Use action='clone' for new repos, action='pull' to update existing repos.",
		impl.Invoke,
	)
}

type GitCloneAction string

const (
	GitCloneActionClone GitCloneAction = "clone"
	GitCloneActionPull  GitCloneAction = "pull"
)

type GitCloneRequest struct {
	Url    string         `json:"url" jsonschema:"description=The URL of the repository to clone"`
	Action GitCloneAction `json:"action" jsonschema:"description=The action to perform, 'clone' or 'pull'"`
}

type GitCloneResponse struct {
	Message      string `json:"message" jsonschema:"description=Success message"`
	Path         string `json:"path,omitempty" jsonschema:"description=Relative path to the cloned repository (use this with search_files and read_file)"`
	Organization string `json:"organization,omitempty" jsonschema:"description=GitHub organization or user name"`
	Repository   string `json:"repository,omitempty" jsonschema:"description=Repository name"`
	NextSteps    string `json:"next_steps,omitempty" jsonschema:"description=Suggested next actions to explore the repository"`
	Error        string `json:"error,omitempty" jsonschema:"description=Error message if operation failed"`
}

func (g *GitCloneImpl) Invoke(ctx context.Context, req *GitCloneRequest) (*GitCloneResponse, error) {
	res := &GitCloneResponse{}

	if req.Url == "" {
		res.Error = "URL cannot be empty"
		return res, nil
	}

	valid, cloneURL := isValidGitURL(req.Url)
	if !valid {
		res.Error = fmt.Sprintf("Invalid Git URL format: %s", req.Url)
		return res, nil
	}

	repoDir, repoName := extractRepoDir(cloneURL)
	repoDir = filepath.Join(g.config.BaseDir, repoDir)
	repoPath := filepath.Join(repoDir, repoName)

	if err := os.MkdirAll(g.config.BaseDir, 0755); err != nil {
		res.Error = fmt.Sprintf("Failed to create directory: %v", err)
		return res, nil
	}

	if req.Action == GitCloneActionClone {
		if _, err := os.Stat(repoPath); err == nil {
			res.Error = "Repository already exists"
			return res, nil
		}

		// Use go-git (pure Go implementation, no external git required!)
		_, err := git.PlainCloneContext(ctx, repoPath, false, &git.CloneOptions{
			URL:           cloneURL,
			Depth:         1,
			SingleBranch:  true,
			ReferenceName: plumbing.HEAD,
			Progress:      nil, // Could add progress reporting
		})

		if err != nil {
			res.Error = fmt.Sprintf("Clone failed: %v", err)
			return res, nil
		}
	} else if req.Action == GitCloneActionPull {
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			res.Error = fmt.Sprintf("repo does not exist: %s", repoPath)
			return res, nil
		}

		// Open existing repository
		repo, err := git.PlainOpen(repoPath)
		if err != nil {
			res.Error = fmt.Sprintf("Failed to open repo: %v", err)
			return res, nil
		}

		// Get the working tree
		w, err := repo.Worktree()
		if err != nil {
			res.Error = fmt.Sprintf("Failed to get worktree: %v", err)
			return res, nil
		}

		// Pull latest changes
		err = w.PullContext(ctx, &git.PullOptions{
			RemoteName: "origin",
			Progress:   nil,
		})

		if err != nil && err != git.NoErrAlreadyUpToDate {
			res.Error = fmt.Sprintf("Pull failed: %v", err)
			return res, nil
		}
	}

	// Get relative path from current directory
	relativePath, err := filepath.Rel(".", repoPath)
	if err != nil {
		// Fallback to the path relative to base directory
		relativePath = filepath.Join(g.config.BaseDir, repoDir, repoName)
	}

	res.Message = fmt.Sprintf("Successfully %sd repository to %s", req.Action, relativePath)
	res.Path = relativePath
	res.Organization = repoDir
	res.Repository = repoName
	res.NextSteps = fmt.Sprintf("IMPORTANT: Use the EXACT path '%s' with all file tools. Examples:\n- search_files(path='%s', pattern='**/*.go')\n- read_file(path='%s/README.md')\n- search_files(path='%s', contains='function')", relativePath, relativePath, relativePath, relativePath)

	return res, nil
}

func isValidGitURL(url string) (bool, string) {
	cleanURL := strings.TrimSuffix(url, ".git")

	parts := strings.Split(cleanURL, "/")
	if len(parts) < 2 {
		return false, ""
	}

	var standardURL string
	switch {
	case strings.HasPrefix(url, "git@"):
		if strings.Contains(url, ":") {
			return true, withGit(url)
		}
		return false, ""

	case strings.HasPrefix(url, "http://"), strings.HasPrefix(url, "https://"):
		return true, withGit(url)

	default:
		standardURL = "https://" + withGit(url)
	}

	return true, standardURL
}

func withGit(url string) string {
	if !strings.HasSuffix(url, ".git") {
		url += ".git"
	}
	return url
}

func extractRepoDir(url string) (string, string) {
	parts := strings.Split(url, "/")
	repoDir := parts[len(parts)-2]
	repoName := strings.TrimSuffix(parts[len(parts)-1], ".git")
	return repoDir, repoName
}
