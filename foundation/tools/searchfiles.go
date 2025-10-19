package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type SearchFilesRequest struct {
	Path     string `json:"path" jsonschema:"description=Directory path to search in (default: current directory)"`
	Pattern  string `json:"pattern,omitempty" jsonschema:"description=Glob pattern to match files (e.g. **/*.go for all Go files, *.md for markdown). Easier than regex!"`
	Filter   string `json:"filter,omitempty" jsonschema:"description=Regex pattern to filter file names if pattern is not enough"`
	Contains string `json:"contains,omitempty" jsonschema:"description=Regex pattern to search inside file contents. Returns line numbers and snippets."`
}

type FileMatch struct {
	File       string   `json:"file" jsonschema:"description=Path to the file"`
	Lines      []int    `json:"lines,omitempty" jsonschema:"description=Line numbers where matches were found (only for content search)"`
	Snippets   []string `json:"snippets,omitempty" jsonschema:"description=Code snippets showing the matches with line numbers"`
	TotalLines int      `json:"total_lines,omitempty" jsonschema:"description=Total lines in the file (only for content search)"`
}

type SearchFilesResponse struct {
	Matches []FileMatch `json:"matches" jsonschema:"description=Files that match the search criteria with location details"`
	Error   string      `json:"error,omitempty" jsonschema:"description=Error message if search failed"`
}

func NewSearchFilesTool(ctx context.Context) (tool.BaseTool, error) {
	return utils.InferTool(
		"search_files",
		"Search for files by glob pattern, regex filter, or content. Returns FULL file paths that you can use directly with read_file or edit_go_file. When searching content with 'contains', returns exact line numbers and code snippets. Example: search_files(path='repos/myrepo', contains='function.*Handle') returns {file: 'repos/myrepo/api/handlers.go', lines: [45], snippets: [...]} - use that EXACT file path in your next tool call.",
		func(ctx context.Context, req *SearchFilesRequest) (*SearchFilesResponse, error) {
			dir := "."
			if req.Path != "" {
				dir = req.Path
			}

			// Verify the directory exists
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return &SearchFilesResponse{
					Error: fmt.Sprintf("directory '%s' does not exist. If you cloned a repo, use the 'path' field from the gitclone response (e.g. 'repos/org/repo').", dir),
				}, nil
			}

			var matches []FileMatch
			skipDirs := []string{"vendor", ".git", "node_modules", ".venv", ".idea", ".vscode"}

			// If glob pattern is provided, use it first (faster and easier!)
			if req.Pattern != "" {
				pattern := filepath.Join(dir, req.Pattern)
				fileMatches, err := doublestar.FilepathGlob(pattern)
				if err != nil {
					return &SearchFilesResponse{
						Error: fmt.Sprintf("invalid glob pattern: %v", err),
					}, nil
				}

				// Apply contains filter if provided
				for _, match := range fileMatches {
					skip := false
					for _, skipDir := range skipDirs {
						if strings.Contains(match, skipDir) {
							skip = true
							break
						}
					}
					if skip {
						continue
					}

					if match == "." {
						continue
					}

					// If searching content, return detailed match info
					if req.Contains != "" {
						fileMatch := searchFileContent(match, req.Contains)
						if fileMatch != nil {
							fileMatch.File = match
							matches = append(matches, *fileMatch)
						}
					} else {
						// Just file name matching
						matches = append(matches, FileMatch{File: match})
					}
				}

				return &SearchFilesResponse{Matches: matches}, nil
			}

			// Fallback to WalkDir with regex filter
			err := filepath.WalkDir(dir, func(path string, info fs.DirEntry, err error) error {
				if err != nil {
					if err == filepath.SkipDir {
						return nil
					}
					return err
				}

				for _, skipDir := range skipDirs {
					if strings.Contains(path, skipDir) {
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
				}

				if path == "." || path == dir {
					return nil
				}

				if req.Filter != "" {
					if matched, _ := regexp.MatchString(req.Filter, path); !matched {
						return nil
					}
				}

				// If searching content, return detailed match info
				if req.Contains != "" {
					if info.IsDir() {
						return nil
					}
					fileMatch := searchFileContent(path, req.Contains)
					if fileMatch != nil {
						fileMatch.File = path
						matches = append(matches, *fileMatch)
					}
					return nil
				}

				// Just file/dir name matching
				if info.IsDir() {
					matches = append(matches, FileMatch{File: path + "/"})
				} else {
					matches = append(matches, FileMatch{File: path})
				}

				return nil
			})

			if err != nil {
				return &SearchFilesResponse{
					Error: fmt.Sprintf("search failed: %v", err),
				}, nil
			}

			return &SearchFilesResponse{Matches: matches}, nil
		},
	)
}

// searchFileContent searches for a regex pattern in a file and returns match details
func searchFileContent(filePath string, pattern string) *FileMatch {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	var matchedLines []int
	var snippets []string

	for i, line := range lines {
		if re.MatchString(line) {
			lineNum := i + 1
			matchedLines = append(matchedLines, lineNum)

			// Include context: 2 lines before and after
			start := i - 2
			if start < 0 {
				start = 0
			}
			end := i + 3
			if end > len(lines) {
				end = len(lines)
			}

			var snippetLines []string
			for j := start; j < end; j++ {
				prefix := "  "
				if j == i {
					prefix = "→ " // Mark the actual match
				}
				snippetLines = append(snippetLines, fmt.Sprintf("%s%4d|%s", prefix, j+1, lines[j]))
			}
			snippets = append(snippets, strings.Join(snippetLines, "\n"))
		}
	}

	if len(matchedLines) == 0 {
		return nil
	}

	return &FileMatch{
		Lines:      matchedLines,
		Snippets:   snippets,
		TotalLines: len(lines),
	}
}
