package tools

import (
	"context"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type EditFileRequest struct {
	Path      string `json:"path" jsonschema:"description=Relative path to the Go file to edit"`
	OldString string `json:"old_string" jsonschema:"description=Exact text to find and replace. Include 3-5 lines of surrounding context to ensure uniqueness. Must match exactly including whitespace."`
	NewString string `json:"new_string" jsonschema:"description=Replacement text. Use empty string to delete the old_string."`
}

type EditFileResponse struct {
	Message string `json:"message" jsonschema:"description=Success message describing what was changed"`
	Error   string `json:"error,omitempty" jsonschema:"description=Error message if edit failed"`
}

func NewEditFileTool(ctx context.Context) (tool.BaseTool, error) {
	return utils.InferTool(
		"edit_go_file",
		"Edit Go source code by replacing exact text. Provide old_string (the exact text to find, with 3-5 lines of context) and new_string (the replacement). The text must match EXACTLY including all whitespace and indentation. Use the full file path from search_files results. Automatically validates syntax and formats with gofmt.",
		func(ctx context.Context, req *EditFileRequest) (*EditFileResponse, error) {
			if req.Path == "" {
				return &EditFileResponse{Error: "path cannot be empty"}, nil
			}

			if req.OldString == "" {
				return &EditFileResponse{Error: "old_string cannot be empty"}, nil
			}

			content, err := os.ReadFile(req.Path)
			if err != nil {
				if os.IsNotExist(err) {
					return &EditFileResponse{
						Error: fmt.Sprintf("file '%s' not found. If from a cloned repo, use the FULL path from search_files result (e.g. 'repos/org/repo/file.go', not just 'file.go').", req.Path),
					}, nil
				}
				return &EditFileResponse{
					Error: fmt.Sprintf("failed to read file '%s': %v", req.Path, err),
				}, nil
			}

			originalContent := string(content)

			if !strings.Contains(originalContent, req.OldString) {
				return &EditFileResponse{
					Error: fmt.Sprintf("old_string not found in file '%s'. The text must match EXACTLY including all whitespace, tabs, and indentation. First read the file with read_file to see the exact content, then copy the lines you want to change (with 3-5 lines of context) as old_string.", req.Path),
				}, nil
			}

			occurrences := strings.Count(originalContent, req.OldString)
			if occurrences > 1 {
				return &EditFileResponse{
					Error: fmt.Sprintf("old_string appears %d times in file '%s'. Add more surrounding lines (before/after) to make it unique. Include function signatures, struct definitions, or other unique context.", occurrences, req.Path),
				}, nil
			}

			modifiedContent := strings.Replace(originalContent, req.OldString, req.NewString, 1)

			fset := token.NewFileSet()
			_, err = parser.ParseFile(fset, req.Path, modifiedContent, parser.ParseComments)
			if err != nil {
				return &EditFileResponse{
					Error: fmt.Sprintf("syntax error after modification in '%s': %v. The new_string likely has incorrect syntax or indentation. Check brackets, semicolons, and ensure proper Go syntax. Read the file again to verify the context.", req.Path, err),
				}, nil
			}

			formattedContent, err := format.Source([]byte(modifiedContent))
			if err != nil {
				formattedContent = []byte(modifiedContent)
			}

			err = os.WriteFile(req.Path, formattedContent, 0644)
			if err != nil {
				return &EditFileResponse{
					Error: fmt.Sprintf("failed to write file: %v", err),
				}, nil
			}

			oldLen := len(req.OldString)
			newLen := len(req.NewString)
			var action string
			if newLen == 0 {
				action = fmt.Sprintf("Deleted %d characters", oldLen)
			} else if oldLen > newLen {
				action = fmt.Sprintf("Replaced %d characters with %d characters (-%d)", oldLen, newLen, oldLen-newLen)
			} else if newLen > oldLen {
				action = fmt.Sprintf("Replaced %d characters with %d characters (+%d)", oldLen, newLen, newLen-oldLen)
			} else {
				action = fmt.Sprintf("Replaced %d characters", oldLen)
			}

			return &EditFileResponse{
				Message: fmt.Sprintf("✅ %s in %s", action, req.Path),
			}, nil
		},
	)
}
