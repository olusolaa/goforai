package tools

import (
	"bufio"
	"context"
	"fmt"
	"github.com/cloudwego/eino/components/tool/utils"
	"io"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
)

// ReadFileRequest defines the parameters for reading a file.
type ReadFileRequest struct {
	Path      string `json:"path" jsonschema:"description=The relative path of the file to read (e.g. 'main.go' or 'pkg/handler/handler.go')"`
	StartLine *int   `json:"start_line,omitempty" jsonschema:"description=Optional: line number to start reading from (1-indexed). Efficient for large files."`
	EndLine   *int   `json:"end_line,omitempty" jsonschema:"description=Optional: line number to stop reading at (inclusive). Efficient for large files."`
}

// ReadFileResponse contains the file contents and metadata.
type ReadFileResponse struct {
	Content    string `json:"content" jsonschema:"description=The contents of the file with line numbers."`
	TotalLines int    `json:"total_lines" jsonschema:"description=Total number of lines in the file."`
	FileSize   int64  `json:"file_size" jsonschema:"description=File size in bytes."`
	StartLine  int    `json:"start_line" jsonschema:"description=First line number that was read."`
	EndLine    int    `json:"end_line" jsonschema:"description=Last line number that was read."`
	Error      string `json:"error,omitempty" jsonschema:"description=Error message if read failed."`
}

// maxLinesToRead sets a safety limit to prevent an LLM from requesting an enormous chunk of a file.
const maxLinesToRead = 5000

// NewReadFileTool creates a new file reading tool for the agent.
func NewReadFileTool(ctx context.Context) (tool.BaseTool, error) {
	return utils.InferTool(
		"read_file",
		"Read the contents of a file with line numbers. This tool is memory-efficient and can safely read slices of very large files using start_line and end_line. Returns metadata (total lines, size) to help decide which parts of a file to read.",
		func(ctx context.Context, req *ReadFileRequest) (*ReadFileResponse, error) {
			if req.Path == "" {
				return &ReadFileResponse{Error: "path cannot be empty"}, nil
			}

			// 1. Perform pre-flight checks with os.Stat first.
			fileInfo, err := os.Stat(req.Path)
			if err != nil {
				if os.IsNotExist(err) {
					return &ReadFileResponse{
						Error: fmt.Sprintf("file '%s' not found. Use search_files to find the correct path.", req.Path),
					}, nil
				}
				return &ReadFileResponse{Error: fmt.Sprintf("failed to get file info for '%s': %v", req.Path, err)}, nil
			}

			if fileInfo.IsDir() {
				return &ReadFileResponse{Error: fmt.Sprintf("path '%s' is a directory, not a file", req.Path)}, nil
			}

			// 2. Open the file for stream-based reading.
			file, err := os.Open(req.Path)
			if err != nil {
				return &ReadFileResponse{Error: fmt.Sprintf("failed to open file '%s': %v", req.Path, err)}, nil
			}
			defer file.Close()

			// 3. Process the file line-by-line to avoid loading it all into memory.
			return processFileLines(file, req, fileInfo)
		},
	)
}

func processFileLines(reader io.Reader, req *ReadFileRequest, fileInfo os.FileInfo) (*ReadFileResponse, error) {
	startLine := 1
	if req.StartLine != nil {
		startLine = *req.StartLine
	}
	// If end_line is not provided, we read until the end of the file.
	endLine := -1 // Use -1 to signify no upper limit
	if req.EndLine != nil {
		endLine = *req.EndLine
	}

	// Validate line numbers
	if startLine < 1 {
		startLine = 1
	}
	if endLine != -1 && endLine < startLine {
		return &ReadFileResponse{Error: fmt.Sprintf("end_line %d is before start_line %d", endLine, startLine)}, nil
	}

	// Safety check on the number of lines to read
	if endLine != -1 && (endLine-startLine+1) > maxLinesToRead {
		endLine = startLine + maxLinesToRead - 1
	}

	scanner := bufio.NewScanner(reader)
	var contentBuilder strings.Builder
	var totalLines, linesRead int
	var actualEndLine int

	for scanner.Scan() {
		totalLines++
		line := scanner.Text()

		if totalLines >= startLine && (endLine == -1 || totalLines <= endLine) {
			// This is a line we want to include.
			if linesRead > 0 {
				contentBuilder.WriteRune('\n')
			}
			fmt.Fprintf(&contentBuilder, "%4d|%s", totalLines, line)
			linesRead++
			actualEndLine = totalLines

			// Apply safety break after reading max lines if no end_line was given.
			if endLine == -1 && linesRead >= maxLinesToRead {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return &ReadFileResponse{Error: fmt.Sprintf("error while reading file '%s': %v", req.Path, err)}, nil
	}

	if totalLines > 0 && startLine > totalLines {
		return &ReadFileResponse{
			Error: fmt.Sprintf("start_line %d is beyond file end (total lines: %d)", startLine, totalLines),
		}, nil
	}

	if linesRead == 0 && totalLines > 0 {
		// This can happen if start_line is valid but no end_line is given, and start_line is past the content.
		// Or if start_line equals end_line but the file has fewer lines.
		// We return an empty content string, which is correct.
	}

	return &ReadFileResponse{
		Content:    contentBuilder.String(),
		TotalLines: totalLines,
		FileSize:   fileInfo.Size(),
		StartLine:  startLine,
		EndLine:    actualEndLine,
	}, nil
}
