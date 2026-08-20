package forage

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func parseSimpleIndex(r io.Reader) ([]File, error) {
	tokenizer := html.NewTokenizer(r)
	var files []File
	var current *File

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			if tokenizer.Err() == io.EOF {
				return files, nil
			}
			return nil, tokenizer.Err()

		case html.StartTagToken:
			tn, hasAttr := tokenizer.TagName()
			if string(tn) != "a" || !hasAttr {
				continue
			}
			f := File{}
			for {
				key, val, more := tokenizer.TagAttr()
				switch string(key) {
				case "href":
					href := string(val)
					u, err := url.Parse(href)
					if err == nil {
						frag := u.Fragment
						u.Fragment = ""
						u.RawFragment = ""
						f.URL = u.String()
						if strings.HasPrefix(frag, "sha256=") {
							f.Digests = []Digest{{Algorithm: "sha256", Value: frag[len("sha256="):]}}
						}
					}
				case "data-provenance":
					s := string(val)
					f.ProvenanceURL = &s
				}
				if !more {
					break
				}
			}
			current = &f

		case html.TextToken:
			if current != nil {
				current.Filename += string(tokenizer.Text())
			}

		case html.EndTagToken:
			tn, _ := tokenizer.TagName()
			if string(tn) == "a" && current != nil {
				current.Filename = strings.TrimSpace(current.Filename)
				files = append(files, *current)
				current = nil
			}
		}
	}
}
