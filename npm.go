package forage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
)

type npmVersionMeta struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	Dist    npmDist `json:"dist"`
}

type npmDist struct {
	Tarball   string `json:"tarball"`
	Integrity string `json:"integrity"`
	Shasum    string `json:"shasum"`
}

// NpmLookup queries an npm registry for the distribution file of the given package and version.
func NpmLookup(ctx context.Context, pkg, version string, opts *Options) (*Result, error) {
	registryURL := DefaultNpmRegistryURL
	if opts != nil && opts.RegistryURL != "" {
		registryURL = opts.RegistryURL
	}
	client := opts.httpClient()

	meta, err := fetchNpmVersionMeta(ctx, client, registryURL, pkg, version)
	if err != nil {
		return nil, err
	}

	file := File{
		Filename: path.Base(meta.Dist.Tarball),
		URL:      meta.Dist.Tarball,
	}

	if meta.Dist.Integrity != "" {
		d, err := ParseSRI(meta.Dist.Integrity)
		if err != nil {
			return nil, fmt.Errorf("parsing integrity for %s@%s: %w", pkg, version, err)
		}
		file.Digests = append(file.Digests, d)
	}
	if meta.Dist.Shasum != "" {
		file.Digests = append(file.Digests, Digest{Algorithm: "sha1", Value: meta.Dist.Shasum})
	}

	if opts != nil && opts.FetchProvenance {
		provURL := npmAttestationURL(registryURL, pkg, version)
		prov, err := fetchProvenance(ctx, client, provURL)
		if err != nil {
			errStr := err.Error()
			file.ProvenanceError = &errStr
		} else {
			file.Provenance = prov
		}
	}

	return &Result{
		Package: pkg,
		Version: version,
		Files:   []File{file},
	}, nil
}

func fetchNpmVersionMeta(ctx context.Context, client *http.Client, indexURL, pkg, version string) (*npmVersionMeta, error) {
	u := strings.TrimRight(indexURL, "/") + "/" + pkg + "/" + version
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("package '%s@%s' not found at %s", pkg, version, u)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, u)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var meta npmVersionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing npm metadata: %w", err)
	}

	return &meta, nil
}

func npmAttestationURL(indexURL, pkg, version string) string {
	return strings.TrimRight(indexURL, "/") + "/-/npm/v1/attestations/" + pkg + "@" + version
}
