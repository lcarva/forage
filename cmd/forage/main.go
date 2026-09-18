package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
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
	var username string
	var passwordStdin bool
	var netrcPath string

	pythonCmd := &cobra.Command{
		Use:   "python <package> <version>",
		Short: "Look up a Python package from a PEP 503 simple index.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg, version := args[0], args[1]
			var provider forage.CredentialProvider
			if username != "" || passwordStdin {
				if username == "" {
					return fmt.Errorf("--username is required with --password-stdin")
				}
				password := ""
				if passwordStdin {
					data, err := io.ReadAll(os.Stdin)
					if err != nil {
						return fmt.Errorf("reading password from stdin: %w", err)
					}
					password = strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
				}
				u, err := url.Parse(indexURL)
				if err != nil {
					return fmt.Errorf("parsing index URL: %w", err)
				}
				if u.Scheme == "" || u.Host == "" {
					return fmt.Errorf("index URL must include a scheme and host: %q", indexURL)
				}
				provider = forage.BasicAuthProvider{
					Origin:   strings.ToLower(u.Scheme + "://" + u.Host),
					Username: username,
					Password: password,
				}
			}

			opts := &forage.Options{
				IndexURL:           indexURL,
				FetchProvenance:    fetchProvenance,
				CredentialProvider: provider,
				NetrcPath:          netrcPath,
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
	pythonCmd.Flags().StringVar(&username, "username", "", "HTTP Basic Auth username or token")
	pythonCmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read the HTTP Basic Auth password from stdin")
	pythonCmd.Flags().StringVar(&netrcPath, "netrc", "", "Path to a netrc file")

	var npmRegistryURL string
	var npmFetchProvenance bool

	npmCmd := &cobra.Command{
		Use:   "npm <package> <version>",
		Short: "Look up an npm package from a registry.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg, version := args[0], args[1]
			opts := &forage.Options{
				RegistryURL:     npmRegistryURL,
				FetchProvenance: npmFetchProvenance,
			}
			result, err := forage.NpmLookup(context.Background(), pkg, version, opts)
			if err != nil {
				return err
			}
			return printResult(result, outputJSON)
		},
	}

	npmCmd.Flags().StringVar(&npmRegistryURL, "registry-url", forage.DefaultNpmRegistryURL,
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
					RegistryURL:     purl.Qualifiers.Map()["repository_url"],
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
		for _, d := range f.Digests {
			fmt.Printf("    %s: %s\n", d.Algorithm, d.Value)
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
