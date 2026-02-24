package xss

import "testing"

func TestSanitizeMarkdown(t *testing.T) {
	in := `<script>alert(1)</script><p onclick="x()">ok</p><iframe src="x"></iframe>`
	out := SanitizeMarkdown(in)
	if out == in {
		t.Fatal("expected markdown to be sanitized")
	}
	if contains(out, "script") || contains(out, "iframe") || contains(out, "onclick") {
		t.Fatalf("dangerous content not sanitized: %s", out)
	}
}

func TestStripHTML(t *testing.T) {
	in := `<h1>Hello</h1>   <p>Go  Forum</p>`
	out := StripHTML(in)
	if out != "Hello Go Forum" {
		t.Fatalf("unexpected strip result: %q", out)
	}
}

func TestTruncateText(t *testing.T) {
	out := TruncateText("abcdefghij", 7)
	if out != "abcd..." {
		t.Fatalf("unexpected truncate output: %q", out)
	}
	out = TruncateText("abc", 10)
	if out != "abc" {
		t.Fatalf("short text should remain unchanged: %q", out)
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
