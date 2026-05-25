package main

import (
	"fmt"
	"os"

	"github.com/archguard/project/config"
	"github.com/archguard/project/delivery/cli"
	"github.com/archguard/project/delivery/output"
	c "github.com/archguard/project/shared/constants"
	"github.com/spf13/cobra"
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
		fmt.Fprintln(os.Stderr, output.RenderError(err))
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
