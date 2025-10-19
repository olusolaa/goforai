package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type ReadFileRequest struct {
	Path      string `json:"path" jsonschema:"description=The relative path of the file to read (e.g. 'main.go' or 'pkg/handler/handler.go')"`
	StartLine *int   `json:"start_line,omitempty" jsonschema:"description=Optional: line number to start reading from (1-indexed). Useful for large files."`
	EndLine   *int   `json:"end_line,omitempty" jsonschema:"description=Optional: line number to stop reading at (inclusive). Useful for large files."`
}

type ReadFileResponse struct {
	Content    string `json:"content" jsonschema:"description=The contents of the file with line numbers"`
	TotalLines int    `json:"total_lines" jsonschema:"description=Total number of lines in the file"`
	FileSize   int64  `json:"file_size" jsonschema:"description=File size in bytes"`
	StartLine  int    `json:"start_line" jsonschema:"description=First line number that was read"`
	EndLine    int    `json:"end_line" jsonschema:"description=Last line number that was read"`
	Error      string `json:"error,omitempty" jsonschema:"description=Error message if read failed"`
}

func NewReadFileTool(ctx context.Context) (tool.BaseTool, error) {
	return utils.InferTool(
		"read_file",
		"Read the contents of a file with line numbers. Returns file metadata (total lines, size) to help you decide if you need to read in chunks. For large files (>500 lines), consider using start_line and end_line parameters to read specific ranges, or use search_files first to locate relevant sections.",
		func(ctx context.Context, req *ReadFileRequest) (*ReadFileResponse, error) {
			if req.Path == "" {
				return &ReadFileResponse{Error: "path cannot be empty"}, nil
			}

			content, err := os.ReadFile(req.Path)
			if err != nil {
				if os.IsNotExist(err) {
					return &ReadFileResponse{
						Error: fmt.Sprintf("file '%s' not found. If from a cloned repo, ensure you're using the full path from gitclone response (e.g. 'repos/org/repo/file.go'). Use search_files to find the correct path.", req.Path),
					}, nil
				}
				return &ReadFileResponse{
					Error: fmt.Sprintf("failed to read file %s: %v", req.Path, err),
				}, nil
			}

			fileInfo, err := os.Stat(req.Path)
			if err != nil {
				return &ReadFileResponse{
					Error: fmt.Sprintf("failed to get file info for %s: %v", req.Path, err),
				}, nil
			}

			lines := strings.Split(string(content), "\n")
			totalLines := len(lines)

			startLine := 1
			endLine := totalLines

			if req.StartLine != nil {
				startLine = *req.StartLine
				if startLine < 1 {
					startLine = 1
				}
				if startLine > totalLines {
					return &ReadFileResponse{
						Error: fmt.Sprintf("start_line %d is beyond file length (file has %d lines)", startLine, totalLines),
					}, nil
				}
			}

			if req.EndLine != nil {
				endLine = *req.EndLine
				if endLine > totalLines {
					endLine = totalLines
				}
				if endLine < startLine {
					return &ReadFileResponse{
						Error: fmt.Sprintf("end_line %d is before start_line %d", endLine, startLine),
					}, nil
				}
			}

			selectedLines := lines[startLine-1 : endLine]

			var formattedLines []string
			for i, line := range selectedLines {
				lineNum := startLine + i
				formattedLines = append(formattedLines, fmt.Sprintf("%4d|%s", lineNum, line))
			}

			return &ReadFileResponse{
				Content:    strings.Join(formattedLines, "\n"),
				TotalLines: totalLines,
				FileSize:   fileInfo.Size(),
				StartLine:  startLine,
				EndLine:    endLine,
			}, nil
		},
	)
}
