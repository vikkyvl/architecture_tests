package detector

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	c "github.com/archguard/project/shared/constants"
)

const (
	detectedLanguageFormat = "  Detected language: %s\n"
	detectedFileFormat     = "    %s: %d files\n"
)

type DetectResult struct {
	PrimaryLanguage string         `json:"primary_language"`
	FileCounts      map[string]int `json:"file_counts"`
}

func Detect(root string) DetectResult {
	extCounts := make(map[string]int)

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			for _, s := range c.SkipDirs {
				if info.Name() == s {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		if ext != "" {
			extCounts[ext]++
		}
		return nil
	})

	langCounts := make(map[string]int)
	for ext, count := range extCounts {
		if lang, ok := c.ExtToLang[ext]; ok {
			langCounts[lang] += count
		}
	}

	best, bestCount := c.UnknownValue, 0
	for lang, count := range langCounts {
		if count > bestCount {
			bestCount = count
			best = lang
		}
	}
	return DetectResult{PrimaryLanguage: best, FileCounts: langCounts}
}

func PrintResult(r DetectResult) {
	fmt.Fprintf(os.Stderr, detectedLanguageFormat, r.PrimaryLanguage)
	type lc struct {
		l string
		c int
	}
	var sorted []lc
	for l, n := range r.FileCounts {
		sorted = append(sorted, lc{l, n})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].c > sorted[j].c })
	for _, s := range sorted {
		fmt.Fprintf(os.Stderr, detectedFileFormat, s.l, s.c)
	}
}

func Extensions(language string) []string {
	return c.LangExtensions[language]
}
