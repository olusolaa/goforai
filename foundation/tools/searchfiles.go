package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type SearchFilesRequest struct {
	Path     string `json:"path,omitempty" jsonschema:"description=Directory path to search in (default: current directory '.')."`
	Pattern  string `json:"pattern,omitempty" jsonschema:"description=Glob pattern for files (e.g., '**/*.go', '*.md'). Recommended over regex filter for path matching."`
	Filter   string `json:"filter,omitempty" jsonschema:"description=Regex pattern to filter file paths. Use this for complex matching not possible with glob patterns."`
	Contains string `json:"contains,omitempty" jsonschema:"description=Regex pattern to search inside file contents. Returns line numbers and snippets."`
}

type FileMatch struct {
	File       string   `json:"file" jsonschema:"description=Path to the file."`
	Lines      []int    `json:"lines,omitempty" jsonschema:"description=Line numbers where matches were found (content search only)."`
	Snippets   []string `json:"snippets,omitempty" jsonschema:"description=Code snippets of the matches with surrounding context and line numbers."`
	TotalLines int      `json:"total_lines,omitempty" jsonschema:"description=Total lines in the file (content search only)."`
}

type SearchFilesResponse struct {
	Matches []FileMatch `json:"matches" jsonschema:"description=Files that match the search criteria."`
	Error   string      `json:"error,omitempty" jsonschema:"description=Error message if search failed."`
}

func NewSearchFilesTool(ctx context.Context) (tool.BaseTool, error) {
	return utils.InferTool(
		"search_files",
		"Recursively search for files by glob pattern, regex filter, and content. Returns full file paths for use with other tools. Content searches ('contains') are parallelized for speed and return exact line numbers and code snippets. Example: search_files(path='repos/myrepo', pattern='**/*.go', contains='func.*Error') finds all Go files containing functions with 'Error' in their signature.",
		func(ctx context.Context, req *SearchFilesRequest) (*SearchFilesResponse, error) {
			// 1. Setup and Validation
			dir := req.Path
			if dir == "" {
				dir = "."
			}

			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return &SearchFilesResponse{Error: fmt.Sprintf("directory '%s' does not exist", dir)}, nil
			}

			var filterRe, containsRe *regexp.Regexp
			var err error
			if req.Filter != "" {
				filterRe, err = regexp.Compile(req.Filter)
				if err != nil {
					return &SearchFilesResponse{Error: fmt.Sprintf("invalid regex for 'filter': %v", err)}, nil
				}
			}
			if req.Contains != "" {
				containsRe, err = regexp.Compile(req.Contains)
				if err != nil {
					return &SearchFilesResponse{Error: fmt.Sprintf("invalid regex for 'contains': %v", err)}, nil
				}
			}

			// 2. Gather all candidate file paths
			candidateFiles, err := collectFiles(dir, req.Pattern)
			if err != nil {
				return &SearchFilesResponse{Error: err.Error()}, nil
			}

			// 3. Filter file paths by regex if provided
			filteredFiles := candidateFiles
			if filterRe != nil {
				filteredFiles = []string{}
				for _, file := range candidateFiles {
					if filterRe.MatchString(file) {
						filteredFiles = append(filteredFiles, file)
					}
				}
			}

			// 4. Process files: either just list them or search content
			var matches []FileMatch
			if containsRe == nil {
				// No content search, just return the filtered file list
				for _, file := range filteredFiles {
					matches = append(matches, FileMatch{File: file})
				}
			} else {
				// Concurrent content search
				matches = searchContentsConcurrently(filteredFiles, containsRe)
			}

			return &SearchFilesResponse{Matches: matches}, nil
		},
	)
}

// collectFiles gathers all files, prioritizing glob pattern if available.
func collectFiles(dir, pattern string) ([]string, error) {
	var files []string
	skipDirs := map[string]struct{}{
		"vendor": {}, ".git": {}, "node_modules": {}, ".venv": {}, ".idea": {}, ".vscode": {},
	}

	if pattern != "" {
		// Use fast doublestar globbing
		globPattern := filepath.Join(dir, pattern)
		globMatches, err := doublestar.FilepathGlob(globPattern, doublestar.WithFailOnIOErrors())
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern or IO error: %w", err)
		}
		// Post-filter the glob results for skipped directories and ensure they are files
		for _, match := range globMatches {
			info, err := os.Stat(match)
			if err != nil || info.IsDir() {
				continue // Skip directories or files that disappeared
			}
			if !isPathInSkippedDir(match, skipDirs) {
				files = append(files, match)
			}
		}
	} else {
		// Fallback to a manual walk
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if _, shouldSkip := skipDirs[d.Name()]; shouldSkip {
					return filepath.SkipDir
				}
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed during file walk: %w", err)
		}
	}
	return files, nil
}

func isPathInSkippedDir(path string, skipDirs map[string]struct{}) bool {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	for _, part := range parts {
		if _, shouldSkip := skipDirs[part]; shouldSkip {
			return true
		}
	}
	return false
}

// searchContentsConcurrently uses a worker pool to search files in parallel.
func searchContentsConcurrently(files []string, containsRe *regexp.Regexp) []FileMatch {
	numWorkers := runtime.NumCPU()
	jobs := make(chan string, len(files))
	results := make(chan FileMatch, len(files))
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range jobs {
				if match := searchFileContent(filePath, containsRe); match != nil {
					results <- *match
				}
			}
		}()
	}

	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	wg.Wait()
	close(results)

	matches := make([]FileMatch, 0, len(results))
	for match := range results {
		matches = append(matches, match)
	}
	return matches
}

// searchFileContent searches a single file for a regex pattern.
func searchFileContent(filePath string, re *regexp.Regexp) *FileMatch {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil // Can't read file
	}

	// Skip binary files
	if len(content) > 0 {
		contentType := http.DetectContentType(content)
		if !strings.HasPrefix(contentType, "text/") {
			return nil
		}
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	var matchedLines []int
	var snippets []string
	var lines []string

	// First pass: read all lines into memory. This is necessary for context snippets.
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	for i, line := range lines {
		if re.MatchString(line) {
			lineNum := i + 1
			matchedLines = append(matchedLines, lineNum)

			start := max(0, i-2)
			end := min(len(lines), i+3)

			var snippetLines []string
			for j := start; j < end; j++ {
				prefix := "  "
				if j == i {
					prefix = "→ " // Mark the matched line
				}
				snippetLines = append(snippetLines, fmt.Sprintf("%s%4d| %s", prefix, j+1, lines[j]))
			}
			snippets = append(snippets, strings.Join(snippetLines, "\n"))
		}
	}

	if len(matchedLines) == 0 {
		return nil
	}

	return &FileMatch{
		File:       filePath,
		Lines:      matchedLines,
		Snippets:   snippets,
		TotalLines: len(lines),
	}
}

// Standard library `min` and `max` for Go < 1.21
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
