package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/lcarva/forage"
	packageurl "github.com/package-url/packageurl-go"
	"github.com/spf13/cobra"
)

var supportedPurlTypes = []string{
	packageurl.TypeNPM,
	packageurl.TypePyPi,
}

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

			return printResult(result, outputJSON)
		},
	}

	pythonCmd.Flags().StringVar(&indexURL, "index-url", forage.DefaultIndexURL,
		"PEP 503 simple index URL")
	pythonCmd.Flags().BoolVar(&fetchProvenance, "fetch-provenance", false,
		"Fetch and inline provenance attestation data")

	var npmIndexURL string
	var npmFetchProvenance bool

	npmCmd := &cobra.Command{
		Use:   "npm <package> <version>",
		Short: "Look up an npm package from a registry.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg, version := args[0], args[1]
			opts := &forage.Options{
				IndexURL:        npmIndexURL,
				FetchProvenance: npmFetchProvenance,
			}
			result, err := forage.NpmLookup(context.Background(), pkg, version, opts)
			if err != nil {
				return err
			}
			return printResult(result, outputJSON)
		},
	}

	npmCmd.Flags().StringVar(&npmIndexURL, "index-url", forage.DefaultNpmIndexURL,
		"npm registry URL")
	npmCmd.Flags().BoolVar(&npmFetchProvenance, "fetch-provenance", false,
		"Fetch and inline provenance attestation data")

	var purlFetchProvenance bool

	purlCmd := &cobra.Command{
		Use:   "purl <package-url>",
		Short: fmt.Sprintf("Look up a package using a Package URL (purl). Supported types: %s.", strings.Join(supportedPurlTypes, ", ")),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			purl, err := packageurl.FromString(args[0])
			if err != nil {
				return fmt.Errorf("invalid purl: %w", err)
			}
			if purl.Version == "" {
				return fmt.Errorf("purl must include a version: %s", args[0])
			}

			switch purl.Type {
			case packageurl.TypeNPM:
				fullName := purl.Name
				if purl.Namespace != "" {
					fullName = purl.Namespace + "/" + purl.Name
				}
				opts := &forage.Options{
					IndexURL:        purl.Qualifiers.Map()["repository_url"],
					FetchProvenance: purlFetchProvenance,
				}
				result, err := forage.NpmLookup(context.Background(), fullName, purl.Version, opts)
				if err != nil {
					return err
				}
				return printResult(result, outputJSON)
			case packageurl.TypePyPi:
				opts := &forage.Options{
					IndexURL:        purl.Qualifiers.Map()["repository_url"],
					FetchProvenance: purlFetchProvenance,
				}
				result, err := forage.Lookup(context.Background(), purl.Name, purl.Version, opts)
				if err != nil {
					return err
				}
				return printResult(result, outputJSON)
			default:
				return fmt.Errorf("unsupported purl type %q (supported types: %s)", purl.Type, strings.Join(supportedPurlTypes, ", "))
			}
		},
	}

	purlCmd.Flags().BoolVar(&purlFetchProvenance, "fetch-provenance", false,
		"Fetch and inline provenance attestation data")

	rootCmd.AddCommand(pythonCmd, npmCmd, purlCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func printResult(result *forage.Result, outputJSON bool) error {
	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("%s %s\n\n", result.Package, result.Version)
	for _, f := range result.Files {
		fmt.Printf("  %s\n", f.Filename)
		if f.SHA256 != "" {
			fmt.Printf("    sha256: %s\n", f.SHA256)
		}
		if f.Integrity != "" {
			fmt.Printf("    integrity: %s\n", f.Integrity)
		}
		if f.Shasum != "" {
			fmt.Printf("    shasum: %s\n", f.Shasum)
		}
		prov := "(none)"
		if f.ProvenanceURL != nil {
			prov = *f.ProvenanceURL
		}
		fmt.Printf("    provenance: %s\n", prov)
		if f.ProvenanceError != nil {
			fmt.Printf("    (provenance fetch failed: %s)\n", *f.ProvenanceError)
		} else if f.Provenance != nil {
			summary := forage.FormatProvenance(f.Provenance)
			for line := range strings.SplitSeq(summary, "\n") {
				fmt.Printf("    %s\n", line)
			}
		}
		fmt.Println()
	}
	return nil
}
