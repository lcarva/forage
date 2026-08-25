//go:build integration

package forage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPyPIAttestationVerification(t *testing.T) {
	requireTools(t, "uvx")

	ctx := context.Background()
	result, err := Lookup(ctx, "cryptography", "48.0.0", &Options{
		IndexURL:        DefaultIndexURL,
		FetchProvenance: true,
	})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	var file *File
	for i := range result.Files {
		if result.Files[i].Filename == "cryptography-48.0.0.tar.gz" {
			file = &result.Files[i]
			break
		}
	}
	if file == nil {
		t.Fatal("cryptography-48.0.0.tar.gz not found in results")
	}
	if file.Provenance == nil {
		t.Fatal("no provenance data for cryptography-48.0.0.tar.gz")
	}
	if len(file.Provenance.Attestations) == 0 {
		t.Fatal("no attestations for cryptography-48.0.0.tar.gz")
	}

	dir := t.TempDir()

	bundleBytes, err := json.Marshal(file.Provenance.Attestations[0].Bundle)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	sidecarPath := filepath.Join(dir, "cryptography-48.0.0.tar.gz.publish.attestation")
	if err := os.WriteFile(sidecarPath, bundleBytes, 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	artifactURL, err := pypiDownloadURL(ctx, "cryptography", "48.0.0", "cryptography-48.0.0.tar.gz")
	if err != nil {
		t.Fatalf("get download URL: %v", err)
	}

	artifactPath := filepath.Join(dir, "cryptography-48.0.0.tar.gz")
	if err := downloadFile(ctx, artifactURL, artifactPath); err != nil {
		t.Fatalf("download artifact: %v", err)
	}

	verify := exec.CommandContext(ctx, "uvx", "--prerelease=allow", "pypi-attestations",
		"verify", "attestation",
		"--identity", "https://github.com/pyca/cryptography/.github/workflows/pypi-publish.yml@refs/heads/46.0.x",
		artifactPath,
	)
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("pypi-attestations verify failed: %v\n%s", err, out)
	}
}

func TestNpmAttestationVerification(t *testing.T) {
	requireTools(t, "cosign")

	ctx := context.Background()
	result, err := NpmLookup(ctx, "sigstore", "3.1.0", &Options{
		FetchProvenance: true,
	})
	if err != nil {
		t.Fatalf("NpmLookup: %v", err)
	}

	if len(result.Files) == 0 {
		t.Fatal("no files in result")
	}
	file := &result.Files[0]
	if file.Provenance == nil {
		t.Fatal("no provenance data")
	}
	if len(file.Provenance.Attestations) < 2 {
		t.Fatalf("expected at least 2 attestations, got %d", len(file.Provenance.Attestations))
	}

	slsaAttestation := file.Provenance.Attestations[1]
	if slsaAttestation.PredicateType != "https://slsa.dev/provenance/v1" {
		t.Fatalf("expected SLSA provenance at index 1, got %q", slsaAttestation.PredicateType)
	}

	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.json")
	bundleBytes, err := json.Marshal(slsaAttestation.Bundle)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	if err := os.WriteFile(bundlePath, bundleBytes, 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	artifactPath := filepath.Join(dir, file.Filename)
	if err := downloadFile(ctx, file.URL, artifactPath); err != nil {
		t.Fatalf("download artifact: %v", err)
	}

	verify := exec.CommandContext(ctx, "cosign", "verify-blob-attestation",
		"--bundle", bundlePath,
		"--certificate-identity", "https://github.com/sigstore/sigstore-js/.github/workflows/release.yml@refs/heads/main",
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
		"--type", "https://slsa.dev/provenance/v1",
		artifactPath,
	)
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("cosign verify failed: %v\n%s", err, out)
	}
}

func requireTools(t *testing.T, tools ...string) {
	t.Helper()
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("required tool %q not found in PATH", tool)
		}
	}
}

func pypiDownloadURL(ctx context.Context, pkg, version, filename string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://pypi.org/pypi/"+pkg+"/"+version+"/json", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data struct {
		URLs []struct {
			Filename string `json:"filename"`
			URL      string `json:"url"`
		} `json:"urls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	for _, u := range data.URLs {
		if u.Filename == filename {
			return u.URL, nil
		}
	}
	return "", fmt.Errorf("filename %q not found in PyPI JSON API response", filename)
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
