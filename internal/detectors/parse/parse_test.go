package parse

import "testing"

func TestVersion(t *testing.T) {
	tests := []struct {
		out  string
		want string
	}{
		{"go version go1.22.3 darwin/arm64", "1.22.3"},
		{"Python 3.12.4", "3.12.4"},
		{"v22.3.0", "22.3.0"},
		{"git version 2.39.5 (Apple Git-154)", "2.39.5"},
		{"Redis server v=7.2.4 sha=00000000:0 malloc=libc", "7.2.4"},
		{"Docker version 27.0.3, build 7d4bcd8", "27.0.3"},
		{"ollama version is 0.3.9", "0.3.9"},
		{"npm 10.8.1", "10.8.1"},
		{"1.2.3-beta.1", "1.2.3-beta.1"},
		{"no version here", ""},
		{"", ""},
		// A bare integer is not a version; requiring a dot avoids matching
		// build numbers and years in banner text.
		{"tool 2024 edition", ""},
	}

	for _, tt := range tests {
		if got := Version(tt.out); got != tt.want {
			t.Errorf("Version(%q) = %q, want %q", tt.out, got, tt.want)
		}
	}
}

func TestFirstLine(t *testing.T) {
	tests := []struct {
		out  string
		want string
	}{
		{"  first  \nsecond\n", "first"},
		{"\n\n\nactual content\n", "actual content"},
		{"", ""},
		{"   \n  \n", ""},
	}

	for _, tt := range tests {
		if got := FirstLine(tt.out); got != tt.want {
			t.Errorf("FirstLine(%q) = %q, want %q", tt.out, got, tt.want)
		}
	}
}

func TestLinesSkipsBlanks(t *testing.T) {
	got := Lines("one\n\n  two  \n\n")
	want := []string{"one", "two"}

	if len(got) != len(want) {
		t.Fatalf("Lines() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Lines()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestVersionKeepsPreReleaseButDropsBuildMetadata(t *testing.T) {
	tests := []struct{ out, want string }{
		// helm reports its git hash as build metadata; it is not part of the
		// version anyone means.
		{"v3.15.2+g1a500d5", "3.15.2"},
		{"1.2.3+20240601", "1.2.3"},
		// A pre-release does change which version this is.
		{"1.2.3-rc.1", "1.2.3-rc.1"},
		{"2.0.0-beta", "2.0.0-beta"},
	}

	for _, tt := range tests {
		if got := Version(tt.out); got != tt.want {
			t.Errorf("Version(%q) = %q, want %q", tt.out, got, tt.want)
		}
	}
}
