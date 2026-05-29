package bootstrap

import (
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/archguard/project/config"
	"github.com/archguard/project/infrastructure/detector"
	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

const (
	estimateOverhead        = 2
	estimateClassDeps       = 1.0
	estimateViolations      = 0.1
	estimateReadFileBase    = 0.1  // structural ambiguity, no domain rules
	estimateReadFilePerRule = 0.15 // each domain rule adds ~15% file coverage
	estimateReadFileCap     = 1.0  // cannot exceed one read per file
)

type EstimateOptions struct {
	ProjectPath  string
	RulesPath    string
	MaxToolCalls int
}

func Estimate(opts EstimateOptions) (*models.EstimateResult, error) {
	cfg, err := config.LoadConfig(opts.RulesPath)
	if err != nil {
		return nil, err
	}

	if cfg.Project.Language == "" {
		det := detector.Detect(opts.ProjectPath)
		cfg.Project.Language = det.PrimaryLanguage
	}
	exts := detector.Extensions(cfg.Project.Language)

	filesByLayer := make(map[string]int, len(cfg.Layers))
	for _, l := range cfg.Layers {
		filesByLayer[l.Name] = 0
	}

	var totalFiles, unknownFiles int

	filepath.Walk(opts.ProjectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			for _, d := range c.SkipDirs {
				if info.Name() == d {
					return filepath.SkipDir
				}
			}
			return nil
		}

		rel, err := filepath.Rel(opts.ProjectPath, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if matchesExclude(rel, cfg.Project.Exclude) {
			return nil
		}
		if c.IsSensitivePath(rel) {
			return nil
		}

		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		if len(exts) > 0 && !containsStr(exts, ext) {
			return nil
		}

		totalFiles++
		layer := layerForFile(rel, cfg.Layers)
		if layer == "" {
			unknownFiles++
		} else {
			filesByLayer[layer]++
		}
		return nil
	})

	readFileRatio := math.Min(estimateReadFileCap,
		estimateReadFileBase+float64(len(cfg.DomainContext.Rules))*estimateReadFilePerRule,
	)
	perFile := estimateClassDeps + readFileRatio + estimateViolations

	estimated := estimateOverhead + int(math.Round(float64(totalFiles)*perFile))

	maxCalls := opts.MaxToolCalls
	if maxCalls <= 0 {
		maxCalls = c.DefaultMaxToolCalls
	}
	runsNeeded := 1
	if estimated > maxCalls {
		runsNeeded = int(math.Ceil(float64(estimated) / float64(maxCalls)))
	}

	layers := make([]models.LayerEstimate, 0, len(cfg.Layers))
	for _, l := range cfg.Layers {
		n := filesByLayer[l.Name]
		calls := int(math.Round(float64(n) * perFile))
		layers = append(layers, models.LayerEstimate{Name: l.Name, FileCount: n, ToolCalls: calls})
	}

	return &models.EstimateResult{
		ProjectName:           cfg.Project.Name,
		Language:              cfg.Project.Language,
		TotalFiles:            totalFiles,
		FilesByLayer:          layers,
		UnknownFiles:          unknownFiles,
		EstimatedToolCalls:    estimated,
		MaxToolCalls:          maxCalls,
		RunsNeeded:            runsNeeded,
		SuggestedMaxToolCalls: estimated,
	}, nil
}

func layerForFile(relPath string, layers []config.LayerConfig) string {
	for _, l := range layers {
		for _, p := range l.Paths {
			prefix := filepath.ToSlash(strings.TrimSuffix(p, "/")) + "/"
			if strings.HasPrefix(relPath+"/", prefix) || relPath == strings.TrimSuffix(p, "/") {
				return l.Name
			}
		}
	}
	return ""
}

func matchesExclude(rel string, patterns []string) bool {
	for _, pat := range patterns {
		cleaned := filepath.ToSlash(strings.TrimSuffix(pat, "/**"))
		if strings.HasSuffix(pat, "/**") || strings.HasSuffix(pat, "/") {
			if strings.HasPrefix(rel, cleaned+"/") || rel == cleaned {
				return true
			}
		} else {
			matched, _ := filepath.Match(pat, rel)
			if matched {
				return true
			}
		}
	}
	return false
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
