package forage

import "testing"

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"requests", "requests"},
		{"My-Package", "my-package"},
		{"my_package", "my-package"},
		{"my.package", "my-package"},
		{"My_.Package", "my-package"},
		{"UPPER", "upper"},
		{"a--b__c..d", "a-b-c-d"},
	}
	for _, tt := range tests {
		if got := normalizeName(tt.input); got != tt.want {
			t.Errorf("normalizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMatchesVersion(t *testing.T) {
	tests := []struct {
		filename string
		pkg      string
		version  string
		want     bool
	}{
		// sdist (.tar.gz)
		{"requests-2.32.2.tar.gz", "requests", "2.32.2", true},
		// wheel
		{"requests-2.32.2-py3-none-any.whl", "requests", "2.32.2", true},
		// version prefix should not match longer version
		{"requests-2.32.20.tar.gz", "requests", "2.32.2", false},
		// different version
		{"requests-2.32.3.tar.gz", "requests", "2.32.2", false},
		// normalized package name
		{"My_Package-1.0.tar.gz", "my-package", "1.0", true},
		{"My_Package-1.0-py3-none-any.whl", "My.Package", "1.0", true},
		// no match
		{"other-1.0.tar.gz", "requests", "1.0", false},
		// filename without version part
		{"requests.tar.gz", "requests", "1.0", false},
		// wheel with extra build tag
		{"requests-2.32.5-0-py3-none-any.whl", "requests", "2.32.5-0", true},
	}
	for _, tt := range tests {
		got := matchesVersion(tt.filename, tt.pkg, tt.version)
		if got != tt.want {
			t.Errorf("matchesVersion(%q, %q, %q) = %v, want %v",
				tt.filename, tt.pkg, tt.version, got, tt.want)
		}
	}
}
