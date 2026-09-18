package forage

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Credentials contains credentials for HTTP Basic authentication.
type Credentials struct {
	Username string
	Password string
}

// CredentialProvider returns credentials for a target URL. Providers should
// scope credentials to the hosts for which they are valid.
type CredentialProvider interface {
	Credentials(context.Context, *url.URL) (Credentials, bool, error)
}

// BasicAuthProvider provides Basic credentials for one URL origin.
type BasicAuthProvider struct {
	Origin   string
	Username string
	Password string
}

// Credentials implements CredentialProvider.
func (p BasicAuthProvider) Credentials(_ context.Context, target *url.URL) (Credentials, bool, error) {
	// An empty Origin cannot be scoped to a host, so fail closed rather than
	// acting as a wildcard that would leak credentials to any target.
	if p.Origin == "" || origin(target) != strings.ToLower(p.Origin) {
		return Credentials{}, false, nil
	}
	return Credentials{Username: p.Username, Password: p.Password}, true, nil
}

type netrcProvider struct {
	path     string
	explicit bool
	once     sync.Once
	entries  map[string]Credentials
	err      error
}

func (p *netrcProvider) Credentials(_ context.Context, target *url.URL) (Credentials, bool, error) {
	p.once.Do(func() {
		entries, err := readNetrc(p.path)
		if err == nil {
			p.entries = entries
			return
		}
		// An explicitly requested netrc file must be usable; surface the error.
		if p.explicit {
			p.err = err
			return
		}
		// The implicit platform-default netrc is optional: a missing file is
		// expected and silently ignored. Any other read/parse error should not
		// break lookups that need no auth, but is worth a warning so it stays
		// visible.
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: ignoring netrc file %q: %v\n", p.path, err)
		}
	})
	if p.err != nil {
		return Credentials{}, false, p.err
	}
	// Only honor host-specific entries. A netrc `default` entry is intentionally
	// ignored to avoid sending credentials to public indexes the user did not
	// mean to authenticate against.
	for _, host := range []string{target.Host, target.Hostname()} {
		if creds, ok := p.entries[host]; ok {
			return creds, true, nil
		}
	}
	return Credentials{}, false, nil
}

func (o *Options) credentialProvider() CredentialProvider {
	if o != nil && o.CredentialProvider != nil {
		return o.CredentialProvider
	}
	path := ""
	explicit := false
	if o != nil && o.NetrcPath != "" {
		path = o.NetrcPath
		explicit = true
	} else if home, err := os.UserHomeDir(); err == nil {
		path = filepath.Join(home, ".netrc")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			path = filepath.Join(home, "_netrc")
		}
	}
	if path == "" {
		return nil
	}
	return &netrcProvider{path: path, explicit: explicit}
}

func (o *Options) authenticatedClient() *http.Client {
	client := o.httpClient()
	provider := o.credentialProvider()
	if provider == nil {
		return client
	}
	clone := *client
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clone.Transport = &credentialTransport{base: transport, provider: provider}
	return &clone
}

type credentialTransport struct {
	base     http.RoundTripper
	provider CredentialProvider
}

func (t *credentialTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if clone.URL.User != nil {
		password, _ := clone.URL.User.Password()
		clone.SetBasicAuth(clone.URL.User.Username(), password)
	} else {
		creds, ok, err := t.provider.Credentials(req.Context(), req.URL)
		if err != nil {
			return nil, err
		}
		if ok {
			clone.SetBasicAuth(creds.Username, creds.Password)
		}
	}
	return t.base.RoundTrip(clone)
}

func origin(u *url.URL) string {
	if u == nil {
		return ""
	}
	return strings.ToLower(u.Scheme + "://" + u.Host)
}

func readNetrc(path string) (map[string]Credentials, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tokens, err := scanNetrc(f)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]Credentials)
	for i := 0; i < len(tokens); {
		kind := tokens[i]
		if kind != "machine" && kind != "default" {
			i++
			continue
		}
		i++
		host := "default"
		if kind == "machine" {
			if i >= len(tokens) {
				break
			}
			host = tokens[i]
			i++
		}
		var creds Credentials
		for i < len(tokens) && tokens[i] != "machine" && tokens[i] != "default" {
			if i+1 >= len(tokens) {
				break
			}
			switch tokens[i] {
			case "login":
				creds.Username = tokens[i+1]
			case "password":
				creds.Password = tokens[i+1]
			}
			i += 2
		}
		entries[host] = creds
	}
	return entries, nil
}

func scanNetrc(f *os.File) ([]string, error) {
	scanner := bufio.NewScanner(f)
	var tokens []string
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		for len(line) > 0 {
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if line[0] == '"' || line[0] == '\'' {
				quote := line[0]
				end := strings.IndexByte(line[1:], quote)
				if end < 0 {
					return nil, fmt.Errorf("unterminated quoted value in netrc")
				}
				tokens = append(tokens, line[1:end+1])
				line = line[end+2:]
				continue
			}
			end := strings.IndexAny(line, " \t")
			if end < 0 {
				tokens = append(tokens, line)
				break
			}
			tokens = append(tokens, line[:end])
			line = line[end:]
		}
	}
	return tokens, scanner.Err()
}
