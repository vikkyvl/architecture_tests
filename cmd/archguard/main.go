package main

import (
	"fmt"
	"os"

	"github.com/archguard/project/client/cli"
	"github.com/archguard/project/shared/config"
	c "github.com/archguard/project/shared/constants"
	"github.com/spf13/cobra"
)

const (
	rootShortDescription    = "ArchGuard Analyzer - architectural analyzer tool powered by AI"
	analyzeCommandUse       = "analyze"
	analyzeCommandShort     = "Analyze project architecture and report violations"
	projectFlagName         = "project"
	projectFlagShorthand    = "p"
	projectFlagDefault      = "."
	projectFlagDescription  = "Path to the project codebase"
	rulesFlagName           = "rules"
	rulesFlagShorthand      = "r"
	rulesFlagDescription    = "Path to YAML rules file"
	docsFlagName            = "docs"
	docsFlagShorthand       = "d"
	docsFlagDescription     = "Path to project documentation"
	outputJSONFlagName      = "output-json"
	outputJSONFlagShorthand = "j"
	outputJSONFlagDefault   = "report.json"
	outputJSONFlagDesc      = "JSON report path"
	outputMDFlagName        = "output-md"
	outputMDFlagShorthand   = "m"
	outputMDFlagDefault     = "report.md"
	outputMDFlagDesc        = "Markdown report path"
	providerFlagName        = "provider"
	providerFlagDesc        = "LLM provider: anthropic / gemini / openai"
	apiKeyFlagName          = "api-key"
	apiKeyFlagDesc          = "API key (or use .env)"
	modelFlagName           = "model"
	modelFlagDesc           = "Model override"
	maxToolCallsFlagName    = "max-tool-calls"
	maxToolCallsFlagDesc    = "Max tool calls"
	timeoutFlagName         = "timeout"
	timeoutFlagDesc         = "Analysis timeout"
	interactiveFlagName     = "interactive"
	interactiveFlagDesc     = "Prompt for consent-required tool calls"
)

func main() {
	config.LoadEnv(c.DefaultEnvFile)

	rootCmd := &cobra.Command{
		Use:           c.AppName,
		Short:         rootShortDescription,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.AddCommand(analyzeCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, cli.RenderError(err))
		os.Exit(1)
	}
}

func analyzeCmd() *cobra.Command {
	var opts cli.ReviewOptions

	cmd := &cobra.Command{
		Use:   analyzeCommandUse,
		Short: analyzeCommandShort,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.RunReview(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.ProjectPath, projectFlagName, projectFlagShorthand, projectFlagDefault, projectFlagDescription)
	cmd.Flags().StringVarP(&opts.RulesPath, rulesFlagName, rulesFlagShorthand, "", rulesFlagDescription)
	cmd.Flags().StringVarP(&opts.DocsPath, docsFlagName, docsFlagShorthand, "", docsFlagDescription)
	cmd.Flags().StringVarP(&opts.OutputJSON, outputJSONFlagName, outputJSONFlagShorthand, outputJSONFlagDefault, outputJSONFlagDesc)
	cmd.Flags().StringVarP(&opts.OutputMD, outputMDFlagName, outputMDFlagShorthand, outputMDFlagDefault, outputMDFlagDesc)
	cmd.Flags().StringVar(&opts.Provider, providerFlagName, "", providerFlagDesc)
	cmd.Flags().StringVar(&opts.APIKey, apiKeyFlagName, "", apiKeyFlagDesc)
	cmd.Flags().StringVar(&opts.Model, modelFlagName, "", modelFlagDesc)
	cmd.Flags().IntVar(&opts.MaxToolCalls, maxToolCallsFlagName, c.DefaultMaxToolCalls, maxToolCallsFlagDesc)
	cmd.Flags().DurationVar(&opts.Timeout, timeoutFlagName, c.DefaultTimeout, timeoutFlagDesc)
	cmd.Flags().BoolVar(&opts.Interactive, interactiveFlagName, true, interactiveFlagDesc)
	_ = cmd.MarkFlagRequired(rulesFlagName)

	return cmd
}
