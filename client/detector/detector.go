package detector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"

	c "github.com/archguard/project/shared/constants"
)

const (
	detectedLanguageFormat = "  Detected language: %s\n"
	detectedFileFormat     = "    %s: %d files (confidence %.0f%%)\n"
	detectorTimeout        = 30 * time.Second
	detectorMaxFileBytes   = 10 * 1024 * 1024
	detectorAcceptScore    = 0.99
	fallbackConfidence     = 0.5
	emptyTreeScore         = 0.0
	smallFileBytes         = 200
	minNamedNodesToTrust   = 5
)

type DetectResult struct {
	PrimaryLanguage string             `json:"primary_language"`
	FileCounts      map[string]int     `json:"file_counts"`
	Confidence      map[string]float64 `json:"confidence,omitempty"`
}

func Detect(root string) DetectResult {
	ctx, cancel := context.WithTimeout(context.Background(), detectorTimeout)
	defer cancel()

	langCounts := make(map[string]int)
	langScores := make(map[string][]float64)

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if info.IsDir() {
			if shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		if ext == "" {
			return nil
		}
		candidate, ok := c.ExtToLang[ext]
		if !ok {
			return nil
		}

		lang, score := attribute(ctx, path, info, ext, candidate)
		if lang == "" {
			return nil
		}
		langCounts[lang]++
		langScores[lang] = append(langScores[lang], score)
		return nil
	})

	return DetectResult{
		PrimaryLanguage: pickPrimary(langCounts),
		FileCounts:      langCounts,
		Confidence:      averageScores(langScores),
	}
}

func shouldSkipDir(name string) bool {
	for _, s := range c.SkipDirs {
		if name == s {
			return true
		}
	}
	return false
}

type grammarFit struct {
	lang  string
	score float64
	named int
}

func attribute(ctx context.Context, path string, info os.FileInfo, ext, candidate string) (string, float64) {
	if info.Size() > detectorMaxFileBytes {
		return candidate, fallbackConfidence
	}
	candidateGrammar := grammarFor(ext, candidate)
	if candidateGrammar == nil {
		return candidate, fallbackConfidence
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0
	}

	score, named := parseStats(ctx, candidateGrammar, data)
	if score >= detectorAcceptScore && (info.Size() < smallFileBytes || named >= minNamedNodesToTrust) {
		return candidate, score
	}
	best := grammarFit{lang: candidate, score: score, named: named}
	for otherLang, otherGrammar := range LanguageGrammars {
		if otherLang == candidate {
			continue
		}
		s, n := parseStats(ctx, otherGrammar, data)
		fit := grammarFit{lang: otherLang, score: s, named: n}
		if betterFit(fit, best) {
			best = fit
		}
	}
	if best.score <= emptyTreeScore {
		return candidate, fallbackConfidence
	}
	return best.lang, best.score
}

func betterFit(candidate, current grammarFit) bool {
	candidatePass := candidate.score >= detectorAcceptScore
	currentPass := current.score >= detectorAcceptScore
	switch {
	case candidatePass && !currentPass:
		return true
	case !candidatePass && currentPass:
		return false
	case candidatePass && currentPass:
		return candidate.named > current.named
	default:
		return candidate.score > current.score
	}
}

func grammarFor(ext, lang string) *sitter.Language {
	if g, ok := ExtraExtensionGrammars[ext]; ok {
		return g
	}
	return LanguageGrammars[lang]
}

func parseStats(ctx context.Context, lang *sitter.Language, source []byte) (float64, int) {
	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree, err := parser.ParseCtx(ctx, nil, source)
	if err != nil || tree == nil {
		return emptyTreeScore, 0
	}
	defer tree.Close()
	root := tree.RootNode()
	if root == nil {
		return emptyTreeScore, 0
	}
	total, errs, named := nodeStats(root)
	if total == 0 {
		return emptyTreeScore, 0
	}
	return 1.0 - float64(errs)/float64(total), named
}

func nodeStats(n *sitter.Node) (total, errs, named int) {
	total = 1
	if n.IsError() || n.IsMissing() {
		errs = 1
	} else if n.IsNamed() {
		named = 1
	}
	count := int(n.ChildCount())
	for i := 0; i < count; i++ {
		t, e, m := nodeStats(n.Child(i))
		total += t
		errs += e
		named += m
	}
	return
}

func averageScores(scores map[string][]float64) map[string]float64 {
	out := make(map[string]float64, len(scores))
	for lang, values := range scores {
		if len(values) == 0 {
			continue
		}
		var sum float64
		for _, v := range values {
			sum += v
		}
		out[lang] = sum / float64(len(values))
	}
	return out
}

func pickPrimary(counts map[string]int) string {
	best, bestCount := c.UnknownValue, 0
	for lang, count := range counts {
		if count > bestCount || (count == bestCount && best != c.UnknownValue && lang < best) {
			bestCount = count
			best = lang
		}
	}
	return best
}

func PrintResult(r DetectResult) {
	fmt.Fprintf(os.Stderr, detectedLanguageFormat, r.PrimaryLanguage)
	type lc struct {
		l    string
		n    int
		conf float64
	}
	var sorted []lc
	for l, n := range r.FileCounts {
		sorted = append(sorted, lc{l, n, r.Confidence[l]})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].n > sorted[j].n
	})
	for _, s := range sorted {
		fmt.Fprintf(os.Stderr, detectedFileFormat, s.l, s.n, s.conf*100)
	}
}

func Extensions(language string) []string {
	return c.LangExtensions[language]
}
