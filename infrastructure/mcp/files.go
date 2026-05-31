package mcp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
)

type fileEntry struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Extension string `json:"extension"`
	Size      int64  `json:"size"`
}

func (r *Resolver) walkProjectFiles(fn func(rel string, info os.FileInfo)) {
	err := filepath.Walk(r.projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, warnSkipInaccessiblePath, err)
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), hiddenPathPrefix) {
				return filepath.SkipDir
			}
			for _, d := range c.SkipDirs {
				if info.Name() == d {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, err := filepath.Rel(r.projectPath, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, warnSkipInaccessiblePath, err)
			return nil
		}
		if c.IsSensitivePath(rel) {
			return nil
		}
		ext := strings.TrimPrefix(filepath.Ext(path), hiddenPathPrefix)
		if len(r.extensions) > 0 && !slices.Contains(r.extensions, ext) {
			return nil
		}
		fn(rel, info)
		return nil
	})
	if err != nil {
		return
	}
}

func (r *Resolver) projectStructure() (string, error) {
	var paths []string
	r.walkProjectFiles(func(rel string, _ os.FileInfo) {
		paths = append(paths, rel)
	})
	return marshalToolResult(toolResultProjectStructure, map[string]interface{}{
		c.JSONKeyRoot:  r.projectPath,
		c.JSONKeyFiles: paths,
		c.JSONKeyTotal: len(paths),
	})
}

func readSourceFile(fullPath, displayPath string) ([]byte, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperrors.NotFound(fmt.Sprintf(errFileNotFoundDisplay, displayPath))
		}
		return nil, apperrors.Wrap(apperrors.KindInternal, operationReadFile, fmt.Sprintf(errReadFile, displayPath, err), err)
	}
	return data, nil
}

func (r *Resolver) readFile(relPath string) (string, bool, error) {
	if relPath == "" {
		return "", false, apperrors.Validation(errPathRequired)
	}
	clean, err := cleanProjectPath(relPath)
	if err != nil {
		return "", false, err
	}
	full := filepath.Join(r.projectPath, clean)
	if err := ensureWithinProject(r.projectPath, full); err != nil {
		return "", false, err
	}

	if r.previouslyAnalyzed[clean] {
		return fmt.Sprintf(readFilePrevRunStubFmt, relPath), true, nil
	}
	if stub, ok := r.checkReadFileDedup(clean, relPath); ok {
		return stub, true, nil
	}

	data, err := readSourceFile(full, relPath)
	if err != nil {
		return "", false, err
	}
	if len(data) > maxReadFileBytes {
		return "", false, apperrors.Validation(fmt.Sprintf(errFileTooLarge, len(data), maxReadFileBytes))
	}

	originalBytes := len(data)
	softCap := r.maxFileBytes
	var truncationSuffix string
	if softCap > 0 && originalBytes > softCap {
		data = data[:softCap]
		truncationSuffix = fmt.Sprintf(maxFileBytesSuffixFmt, softCap, originalBytes-softCap)
	}

	lines := strings.Split(string(data), lineSeparator)
	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, lineNumberFormat, i+1, line)
	}
	if truncationSuffix != "" {
		sb.WriteString(truncationSuffix)
	}
	r.recordReadFile(clean, originalBytes)
	return sb.String(), false, nil
}

func (r *Resolver) checkReadFileDedup(cleanPath, displayPath string) (string, bool) {
	if r.keepWindowTurns <= 0 {
		return "", false
	}
	r.readFilesMu.Lock()
	defer r.readFilesMu.Unlock()
	entry, ok := r.readFiles[cleanPath]
	if !ok {
		return "", false
	}
	if r.readTurn-entry.Turn >= r.keepWindowTurns {
		delete(r.readFiles, cleanPath)
		return "", false
	}
	return fmt.Sprintf(readFileDedupStubFmt, displayPath, entry.Size), true
}

func (r *Resolver) recordReadFile(cleanPath string, size int) {
	r.readFilesMu.Lock()
	defer r.readFilesMu.Unlock()
	r.readFiles[cleanPath] = readFileEntry{Turn: r.readTurn, Size: size}
	r.readTurn++
}

func (r *Resolver) resolveFile(classOrPath string) (string, error) {
	if strings.Contains(classOrPath, pathSeparator) {
		clean := filepath.Clean(classOrPath)
		if strings.HasPrefix(clean, pathTraversalPrefix) || filepath.IsAbs(clean) || c.IsSensitivePath(clean) {
			return "", apperrors.PermissionDenied(fmt.Sprintf(errSensitivePath, classOrPath))
		}
		if _, err := os.Stat(filepath.Join(r.projectPath, clean)); err == nil {
			return clean, nil
		}
	}
	parts := strings.Split(classOrPath, phpNamespaceSeparator)
	if len(parts) > 1 {
		srcRoot := r.srcRoot
		if srcRoot == "" {
			srcRoot = c.DefaultSrcRoot
		}
		ext := c.LangFileExtensions[r.language]
		if ext == "" {
			ext = c.DefaultPHPFile
		}
		rel := filepath.Join(srcRoot, filepath.Join(parts[1:]...)) + ext
		if c.IsSensitivePath(rel) {
			return "", apperrors.PermissionDenied(fmt.Sprintf(errSensitivePath, rel))
		}
		if _, err := os.Stat(filepath.Join(r.projectPath, rel)); err == nil {
			return rel, nil
		}
	}
	return "", apperrors.NotFound(fmt.Sprintf(errFileNotFound, classOrPath))
}

func newFileEntry(rel string, info os.FileInfo) fileEntry {
	return fileEntry{
		Path:      rel,
		Name:      info.Name(),
		Extension: strings.TrimPrefix(filepath.Ext(info.Name()), hiddenPathPrefix),
		Size:      info.Size(),
	}
}

func cleanProjectPath(relPath string) (string, error) {
	clean := filepath.Clean(relPath)
	if strings.HasPrefix(clean, pathTraversalPrefix) || filepath.IsAbs(clean) {
		return "", apperrors.PermissionDenied(errRelativePathRequired)
	}
	if c.IsSensitivePath(clean) {
		return "", apperrors.PermissionDenied(fmt.Sprintf(errSensitivePath, relPath))
	}
	return clean, nil
}

func ensureWithinProject(projectPath, fullPath string) error {
	absProj, err := filepath.Abs(projectPath)
	if err != nil {
		return apperrors.PermissionDenied(fmt.Sprintf("%s: %v", errPathTraversal, err))
	}
	absFile, err := filepath.Abs(fullPath)
	if err != nil {
		return apperrors.PermissionDenied(fmt.Sprintf("%s: %v", errPathTraversal, err))
	}
	rel, err := filepath.Rel(absProj, absFile)
	if err != nil || strings.HasPrefix(rel, pathTraversalPrefix) || filepath.IsAbs(rel) {
		return apperrors.PermissionDenied(errPathTraversal)
	}
	return nil
}
