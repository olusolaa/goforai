package tools

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"golang.org/x/tools/go/ast/astutil"
)

type EditFileRequest struct {
	Path        string `json:"path" jsonschema:"description=Path to the Go file to edit."`
	Operation   string `json:"operation" jsonschema:"description=Type of edit: 'add_import', 'remove_import', 'add_var', 'add_const', 'add_function', or 'replace_code_block'."`
	ImportPath  string `json:"import_path,omitempty" jsonschema:"description=For 'add_import'/'remove_import': the import path (e.g., 'fmt')."`
	ImportAlias string `json:"import_alias,omitempty" jsonschema:"description=For 'add_import': optional alias for the import."`
	VarName     string `json:"var_name,omitempty" jsonschema:"description=For 'add_var'/'add_const': the variable/constant name."`
	VarType     string `json:"var_type,omitempty" jsonschema:"description=For 'add_var'/'add_const': the type (e.g., 'string', 'error'). Optional if value is provided."`
	VarValue    string `json:"var_value,omitempty" jsonschema:"description=For 'add_var'/'add_const': the value expression (e.g., '\"hello\"', 'errors.New(\"not found\")'). Optional."`
	Code        string `json:"code,omitempty" jsonschema:"description=For 'add_function' or 'replace_code_block': The complete and syntactically valid Go code for the new block. IMPORTANT: For 'replace_code_block', this MUST be the full declaration (e.g., the entire function from 'func...' to the final '}', not just the changed lines)."`
	StartLine   *int   `json:"start_line,omitempty" jsonschema:"description=For 'replace_code_block': the first line number of the block to replace (1-indexed)."`
	EndLine     *int   `json:"end_line,omitempty" jsonschema:"description=For 'replace_code_block': the last line number of the block to replace (inclusive)."`
}

type EditFileResponse struct {
	Message string `json:"message" jsonschema:"description=Success message describing the change."`
	Error   string `json:"error,omitempty" jsonschema:"description=Error message if the operation failed."`
}

func NewEditFileTool(ctx context.Context) (tool.BaseTool, error) {
	return utils.InferTool(
		"edit_go_file",
		"Replaces a block of Go code in a file, identified by line numbers. CRITICAL: The 'code' parameter MUST be a complete, self-contained Go declaration (e.g., a full 'func', 'type', or 'var' block). Providing incomplete snippets (like just an 'if' or 'for' loop) WILL FAIL.",
		func(ctx context.Context, req *EditFileRequest) (*EditFileResponse, error) {
			if req.Path == "" {
				return &EditFileResponse{Error: "path cannot be empty"}, nil
			}

			content, perms, err := readFileWithPerms(req.Path)
			if err != nil {
				return &EditFileResponse{Error: err.Error()}, nil
			}

			var modifiedContent []byte
			var message string
			var isASTOperation bool

			switch req.Operation {
			case "add_import", "remove_import", "add_var", "add_const", "add_function":
				isASTOperation = true
			case "replace_code_block":
				isASTOperation = false
				modifiedContent, message, err = replaceCodeBlock(content, req.StartLine, req.EndLine, req.Code)
			default:
				return &EditFileResponse{
					Error: fmt.Sprintf("unknown operation '%s'. Use: add_import, remove_import, add_var, add_const, add_function, replace_code_block", req.Operation),
				}, nil
			}

			if isASTOperation {
				modifiedContent, message, err = performASTOperation(req, content)
			}

			if err != nil {
				return &EditFileResponse{Error: err.Error()}, nil
			}

			// Final safety check: ensure the generated code is still valid Go.
			fset := token.NewFileSet()
			_, err = parser.ParseFile(fset, "", modifiedContent, parser.ParseComments)
			if err != nil {
				return &EditFileResponse{
					Error: fmt.Sprintf("internal error or invalid edit: generated code is syntactically invalid: %v", err),
				}, nil
			}

			// Format the source code before writing.
			formattedContent, err := format.Source(modifiedContent)
			if err != nil {
				return &EditFileResponse{
					Error: fmt.Sprintf("failed to gofmt generated code: %v", err),
				}, nil
			}

			if err := atomicWriteFile(req.Path, formattedContent, perms); err != nil {
				return &EditFileResponse{Error: fmt.Sprintf("failed to write file: %v", err)}, nil
			}

			return &EditFileResponse{
				Message: fmt.Sprintf("✅ %s in %s", message, req.Path),
			}, nil
		},
	)
}

// performASTOperation handles all edits that modify the Go Abstract Syntax Tree.
func performASTOperation(req *EditFileRequest, content []byte) ([]byte, string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, req.Path, content, parser.ParseComments)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse original file: %w", err)
	}

	var changed bool
	var message string

	switch req.Operation {
	case "add_import":
		changed, message, err = addImport(fset, file, req.ImportPath, req.ImportAlias)
	case "remove_import":
		changed, message, err = removeImport(fset, file, req.ImportPath)
	case "add_var":
		changed, message, err = addTopLevelDecl(file, req.VarName, req.VarType, req.VarValue, false)
	case "add_const":
		changed, message, err = addTopLevelDecl(file, req.VarName, req.VarType, req.VarValue, true)
	case "add_function":
		changed, message, err = addFunctionAST(file, req.Code)
	}

	if err != nil {
		return nil, "", err
	}

	// If an idempotent operation resulted in no change, return the original content.
	if !changed {
		return content, message, nil
	}

	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.TabIndent | printer.UseSpaces, Tabwidth: 8}
	if err := cfg.Fprint(&buf, fset, file); err != nil {
		return nil, "", fmt.Errorf("failed to print modified AST: %w", err)
	}

	return buf.Bytes(), message, nil
}

// --- Operation Implementations ---

func addImport(fset *token.FileSet, file *ast.File, importPath, alias string) (bool, string, error) {
	if importPath == "" {
		return false, "", fmt.Errorf("import_path cannot be empty")
	}
	var changed bool
	if alias != "" {
		changed = astutil.AddNamedImport(fset, file, alias, importPath)
	} else {
		changed = astutil.AddImport(fset, file, importPath)
	}

	var msg string
	if !changed {
		msg = fmt.Sprintf("Import '%s' already exists", importPath)
	} else if alias != "" {
		msg = fmt.Sprintf("Added import %s '%s'", alias, importPath)
	} else {
		msg = fmt.Sprintf("Added import '%s'", importPath)
	}
	return changed, msg, nil
}

func removeImport(fset *token.FileSet, file *ast.File, importPath string) (bool, string, error) {
	if importPath == "" {
		return false, "", fmt.Errorf("import_path cannot be empty")
	}
	changed := astutil.DeleteImport(fset, file, importPath)

	msg := fmt.Sprintf("Removed import '%s'", importPath)
	if !changed {
		msg = fmt.Sprintf("Import '%s' not found", importPath)
	}
	return changed, msg, nil
}

func addTopLevelDecl(file *ast.File, name, varType, value string, isConst bool) (bool, string, error) {
	if name == "" {
		return false, "", fmt.Errorf("var_name cannot be empty")
	}
	if varType == "" && value == "" {
		return false, "", fmt.Errorf("either var_type or var_value must be provided")
	}

	keyword, tokType := "var", token.VAR
	if isConst {
		keyword, tokType = "const", token.CONST
	}

	// Check if decl already exists.
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == tokType {
			for _, spec := range genDecl.Specs {
				if vSpec, ok := spec.(*ast.ValueSpec); ok {
					for _, ident := range vSpec.Names {
						if ident.Name == name {
							return false, fmt.Sprintf("%s '%s' already exists", strings.Title(keyword), name), nil
						}
					}
				}
			}
		}
	}

	valueSpec := &ast.ValueSpec{Names: []*ast.Ident{ast.NewIdent(name)}}
	if varType != "" {
		valueSpec.Type = ast.NewIdent(varType)
	}
	if value != "" {
		expr, err := parser.ParseExpr(value)
		if err != nil {
			return false, "", fmt.Errorf("invalid expression for var_value: %w", err)
		}
		valueSpec.Values = []ast.Expr{expr}
	}
	newDecl := &ast.GenDecl{Tok: tokType, Specs: []ast.Spec{valueSpec}}

	// CORRECTED: Directly append the new declaration to the file's declarations slice.
	file.Decls = append(file.Decls, newDecl)
	return true, fmt.Sprintf("Added %s '%s'", keyword, name), nil
}

func addFunctionAST(file *ast.File, code string) (bool, string, error) {
	if code == "" {
		return false, "", fmt.Errorf("code cannot be empty for add_function")
	}

	src := "package p;\n" + code
	fsetFrag := token.NewFileSet()
	fileFrag, err := parser.ParseFile(fsetFrag, "fragment.go", src, 0)
	if err != nil {
		return false, "", fmt.Errorf("invalid Go code provided for function: %w", err)
	}
	if len(fileFrag.Decls) == 0 {
		return false, "", fmt.Errorf("code does not contain a valid function declaration")
	}
	funcDecl, ok := fileFrag.Decls[0].(*ast.FuncDecl)
	if !ok {
		return false, "", fmt.Errorf("code does not appear to be a function declaration")
	}
	funcName := funcDecl.Name.Name

	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == funcName {
			return false, fmt.Sprintf("Function '%s' already exists", funcName), nil
		}
	}

	// CORRECTED: Directly append the new function declaration to the file's declarations slice.
	file.Decls = append(file.Decls, funcDecl)
	return true, fmt.Sprintf("Added function '%s'", funcName), nil
}

func replaceCodeBlock(content []byte, startLine, endLine *int, newText string) ([]byte, string, error) {
	if startLine == nil || endLine == nil {
		return nil, "", fmt.Errorf("start_line and end_line are required for replace_code_block")
	}
	if newText == "" {
		return nil, "", fmt.Errorf("code (new_text) cannot be empty for replace_code_block")
	}

	// Safety check: parse the new text to ensure it's valid Go before modifying the file.
	src := "package p;\n" + newText
	_, err := parser.ParseFile(token.NewFileSet(), "fragment.go", src, parser.ParseComments)
	if err != nil {
		return nil, "", fmt.Errorf("the provided replacement 'code' is not valid Go syntax: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	startIdx, endIdx := *startLine-1, *endLine-1 // Convert to 0-based index

	if startIdx < 0 || startIdx >= len(lines) {
		return nil, "", fmt.Errorf("start_line %d is out of file bounds (1-%d)", *startLine, len(lines))
	}
	if endIdx < 0 || endIdx >= len(lines) || endIdx < startIdx {
		return nil, "", fmt.Errorf("end_line %d is invalid or out of file bounds (1-%d)", *endLine, len(lines))
	}

	var newLines []string
	newLines = append(newLines, lines[:startIdx]...)
	newLines = append(newLines, newText)
	if endIdx+1 < len(lines) {
		newLines = append(newLines, lines[endIdx+1:]...)
	}

	msg := fmt.Sprintf("Replaced code block from line %d to %d", *startLine, *endLine)
	return []byte(strings.Join(newLines, "\n")), msg, nil
}

// --- Robust File I/O Utilities ---

func readFileWithPerms(path string) ([]byte, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to stat file '%s': %w", path, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read file '%s': %w", path, err)
	}
	return content, info.Mode().Perm(), nil
}

func atomicWriteFile(path string, data []byte, perms os.FileMode) (err error) {
	tmpFile, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		if err != nil {
			os.Remove(tmpFile.Name())
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err := os.Chmod(tmpFile.Name(), perms); err != nil {
		return fmt.Errorf("failed to set permissions on temporary file: %w", err)
	}
	if err := os.Rename(tmpFile.Name(), path); err != nil {
		return fmt.Errorf("failed to atomically replace file: %w", err)
	}
	return nil
}
