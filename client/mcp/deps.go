package mcp

import (
	"context"
	"fmt"
	"os"
	"time"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/archguard/project/client/detector"
	mcpimports "github.com/archguard/project/client/mcp/imports"
	c "github.com/archguard/project/shared/constants"
)

const extractDepsParseTimeout = 10 * time.Second

func (r *Resolver) extractDeps(source, lang string) (deps []map[string]string) {
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Fprintf(os.Stderr, warnExtractDepsRecovered, lang, rec)
			deps = nil
		}
	}()

	if lang == "" || source == "" {
		return nil
	}
	grammar, ok := detector.LanguageGrammars[lang]
	if !ok {
		grammar = detector.ExtraExtensionGrammars[lang]
	}
	extractor, hasExtractor := mcpimports.Extractors[lang]
	if grammar == nil || !hasExtractor {
		return nil
	}
	parser := sitter.NewParser()
	parser.SetLanguage(grammar)
	ctx, cancel := context.WithTimeout(context.Background(), extractDepsParseTimeout)
	defer cancel()
	tree, err := parser.ParseCtx(ctx, nil, []byte(source))
	if err != nil || tree == nil {
		return nil
	}
	defer tree.Close()
	root := tree.RootNode()
	if root == nil {
		return nil
	}
	for _, name := range extractor(root, []byte(source)) {
		deps = append(deps, map[string]string{
			c.JSONKeyClass: name,
			c.JSONKeyLayer: r.resolveLayer(name),
			c.JSONKeyType:  c.JSONValueImport,
		})
	}
	return deps
}
