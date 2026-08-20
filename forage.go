package forage

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Provenance holds normalized, ecosystem-agnostic provenance attestation data.
type Provenance struct {
	Publisher    *Publisher    `json:"publisher,omitempty"`
	Attestations []Attestation `json:"attestations"`
}

// Attestation is a single normalized attestation entry.
type Attestation struct {
	MediaType     string          `json:"mediaType"`
	PredicateType string          `json:"predicateType,omitempty"`
	Bundle        json.RawMessage `json:"bundle"`
}

// Publisher describes who published the attestations.
type Publisher struct {
	Kind       string `json:"kind"`
	Repository string `json:"repository,omitempty"`
	Workflow   string `json:"workflow,omitempty"`
}

const DefaultIndexURL = "https://pypi.org/simple/"
const DefaultNpmRegistryURL = "https://registry.npmjs.org"

// Digest represents a single hash digest with its algorithm name and hex-encoded value.
type Digest struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

// ParseSRI parses a Subresource Integrity string (e.g. "sha512-base64data==")
// into its algorithm name and hex-encoded digest value.
func ParseSRI(sri string) (Digest, error) {
	parts := strings.SplitN(sri, "-", 2)
	if len(parts) != 2 {
		return Digest{}, fmt.Errorf("invalid SRI string: %q", sri)
	}
	raw, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return Digest{}, fmt.Errorf("decoding SRI base64 for %s: %w", parts[0], err)
	}
	return Digest{
		Algorithm: parts[0],
		Value:     hex.EncodeToString(raw),
	}, nil
}

// File represents a package distribution file discovered from an index.
type File struct {
	Filename        string      `json:"filename"`
	URL             string      `json:"-"`
	Digests         []Digest    `json:"digests,omitempty"`
	ProvenanceURL   *string     `json:"provenance_url"`
	Provenance      *Provenance `json:"provenance,omitempty"`
	ProvenanceError *string     `json:"provenance_error,omitempty"`
}

// Result holds the lookup results for a package version.
type Result struct {
	Package string `json:"package"`
	Version string `json:"version"`
	Files   []File `json:"files"`
}

// Options configures a Lookup call.
type Options struct {
	IndexURL        string
	RegistryURL     string
	FetchProvenance bool
	HTTPClient      *http.Client
}

func (o *Options) indexURL() string {
	if o != nil && o.IndexURL != "" {
		return o.IndexURL
	}
	return DefaultIndexURL
}

func (o *Options) httpClient() *http.Client {
	if o != nil && o.HTTPClient != nil {
		return o.HTTPClient
	}
	return http.DefaultClient
}

// Lookup queries a PEP 503 simple index for files matching the given package and version.
func Lookup(ctx context.Context, pkg, version string, opts *Options) (*Result, error) {
	indexURL := opts.indexURL()
	client := opts.httpClient()

	body, err := fetchIndex(ctx, client, indexURL, pkg)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	allFiles, err := parseSimpleIndex(body)
	if err != nil {
		return nil, fmt.Errorf("parsing index page: %w", err)
	}

	var matched []File
	for _, f := range allFiles {
		if matchesVersion(f.Filename, pkg, version) {
			matched = append(matched, f)
		}
	}

	if len(matched) == 0 {
		return nil, fmt.Errorf("no files found for %s==%s", pkg, version)
	}

	if opts != nil && opts.FetchProvenance {
		for i := range matched {
			if matched[i].ProvenanceURL == nil {
				continue
			}
			raw, err := fetchProvenance(ctx, client, *matched[i].ProvenanceURL)
			if err != nil {
				errStr := err.Error()
				matched[i].ProvenanceError = &errStr
				continue
			}
			prov, err := normalizePyPIProvenance(raw)
			if err != nil {
				errStr := err.Error()
				matched[i].ProvenanceError = &errStr
			} else {
				matched[i].Provenance = prov
			}
		}
	}

	return &Result{
		Package: pkg,
		Version: version,
		Files:   matched,
	}, nil
}

func fetchIndex(ctx context.Context, client *http.Client, indexURL, pkg string) (io.ReadCloser, error) {
	u := strings.TrimRight(indexURL, "/") + "/" + pkg + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.pypi.simple.v1+html, text/html;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, fmt.Errorf("package '%s' not found at %s", pkg, u)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, u)
	}
	return resp.Body, nil
}

func fetchProvenance(ctx context.Context, client *http.Client, provURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d fetching provenance from %s", resp.StatusCode, provURL)
	}
	return io.ReadAll(resp.Body)
}
