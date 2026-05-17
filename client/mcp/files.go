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
	filepath.Walk(r.projectPath, func(path string, info os.FileInfo, err error) error {
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
		rel, _ := filepath.Rel(r.projectPath, path)
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
}

func (r *Resolver) projectStructure() (string, error) {
	var files []fileEntry
	r.walkProjectFiles(func(rel string, info os.FileInfo) {
		files = append(files, newFileEntry(rel, info))
	})
	return marshalToolResult(toolResultProjectStructure, map[string]interface{}{
		c.JSONKeyRoot:  r.projectPath,
		c.JSONKeyFiles: files,
		c.JSONKeyTotal: len(files),
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

func (r *Resolver) readFile(relPath string) (string, error) {
	if relPath == "" {
		return "", apperrors.Validation(errPathRequired)
	}
	clean, err := cleanProjectPath(relPath)
	if err != nil {
		return "", err
	}
	full := filepath.Join(r.projectPath, clean)
	if err := ensureWithinProject(r.projectPath, full); err != nil {
		return "", err
	}
	data, err := readSourceFile(full, relPath)
	if err != nil {
		return "", err
	}
	if len(data) > maxReadFileBytes {
		return "", apperrors.Validation(fmt.Sprintf(errFileTooLarge, len(data), maxReadFileBytes))
	}
	lines := strings.Split(string(data), lineSeparator)
	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, lineNumberFormat, i+1, line)
	}
	return sb.String(), nil
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
		srcRoot := r.cfg.Project.SrcRoot
		if srcRoot == "" {
			srcRoot = c.DefaultSrcRoot
		}
		ext := c.LangFileExtensions[r.cfg.Project.Language]
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
	absProj, _ := filepath.Abs(projectPath)
	absFile, _ := filepath.Abs(fullPath)
	rel, err := filepath.Rel(absProj, absFile)
	if err != nil || strings.HasPrefix(rel, pathTraversalPrefix) || filepath.IsAbs(rel) {
		return apperrors.PermissionDenied(errPathTraversal)
	}
	return nil
}
