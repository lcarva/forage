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
	var indexURL string
	var outputJSON bool
	var fetchProvenance bool

	cmd := &cobra.Command{
		Use:   "forage <package> <version>",
		Short: "Discover package files, digests, and provenance from package indexes.",
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
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().StringVar(&indexURL, "index-url", forage.DefaultIndexURL,
		"PEP 503 simple index URL")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output JSON")
	cmd.Flags().BoolVar(&fetchProvenance, "fetch-provenance", false,
		"Fetch and inline provenance attestation data")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
