package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/lcarva/forage"
	"github.com/spf13/cobra"
)

func main() {
	var outputJSON bool

	rootCmd := &cobra.Command{
		Use:           "forage",
		Short:         "Discover package files, digests, and provenance from package indexes.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output JSON")

	var indexURL string
	var fetchProvenance bool

	pythonCmd := &cobra.Command{
		Use:   "python <package> <version>",
		Short: "Look up a Python package from a PEP 503 simple index.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg, version := args[0], args[1]

			opts := &forage.Options{
				IndexURL:        indexURL,
				FetchProvenance: fetchProvenance,
			}
			result, err := forage.Lookup(context.Background(), pkg, version, opts)
			if err != nil {
				return err
			}

			if outputJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Printf("%s %s\n\n", result.Package, result.Version)
			for _, f := range result.Files {
				fmt.Printf("  %s\n", f.Filename)
				fmt.Printf("    sha256: %s\n", f.SHA256)
				prov := "(none)"
				if f.ProvenanceURL != nil {
					prov = *f.ProvenanceURL
				}
				fmt.Printf("    provenance: %s\n", prov)
				if f.Provenance != nil {
					summary := forage.FormatProvenance(f.Provenance)
					for _, line := range strings.Split(summary, "\n") {
						fmt.Printf("    %s\n", line)
					}
				}
				fmt.Println()
			}
			return nil
		},
	}

	pythonCmd.Flags().StringVar(&indexURL, "index-url", forage.DefaultIndexURL,
		"PEP 503 simple index URL")
	pythonCmd.Flags().BoolVar(&fetchProvenance, "fetch-provenance", false,
		"Fetch and inline provenance attestation data")

	rootCmd.AddCommand(pythonCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
