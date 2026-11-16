package variables

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseAnnotatedValues(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, text string
		want       []Entry
	}{
		{"leading", "# target domain\nTARGET=example.test", []Entry{{"TARGET", "example.test", "target domain"}}},
		{"inline multiline", "TARGET=host /* first\nsecond */\nNEXT=other", []Entry{{"TARGET", "host", "first\nsecond"}, {"NEXT", "other", ""}}},
		{"inline", "TARGET=mail.example.test // mail host", []Entry{{"TARGET", "mail.example.test", "mail host"}}},
		{"url", "URL=https://example.test/a#fragment", []Entry{{"URL", "https://example.test/a#fragment", ""}}},
		{"block", "/* first\nsecond */\nTARGET=host /* inline */", []Entry{{"TARGET", "host", "first\nsecond\ninline"}}},
		{"quoted", "TEXT=\"a # b // c /* d */\"", []Entry{{"TEXT", "a # b // c /* d */", ""}}},
		{"detached", "// detached\n\nTARGET=host", []Entry{{"TARGET", "host", ""}}},
		{"literal", "TEXT='$(touch /tmp/never) $HOME'", []Entry{{"TEXT", "$(touch /tmp/never) $HOME", ""}}},
		{"escape", `TEXT="a\\b\"c"`, []Entry{{"TEXT", `a\b"c`, ""}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(test.text)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v, %v; want %#v", got, err, test.want)
			}
		})
	}
}

func TestParseRejectsInvalidFilesWithoutEchoingValues(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"TARGET=private\nTARGET=private", "bad-name=private", "TARGET=\"private", "TARGET=\"private\" garbage", "TARGET=", "/* private", "TARGET=\"private\\n\"", "TARGET=wraith::OTHER", "TARGET=private\x1b", strings.Repeat("#", FileLimit+1)} {
		if _, err := Parse(text); err == nil || strings.Contains(err.Error(), "private") {
			t.Fatalf("unsafe diagnostic: %v", err)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, text := range []string{"TARGET=https://example.test // target", "/* note */\nX='value'", ""} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		entries, err := Parse(text)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if !namePattern.MatchString(entry.Name) || entry.Value == "" {
				t.Fatal("invalid parsed entry")
			}
		}
	})
}

func TestInlineCommentContinuationsRejectTerminalControls(t *testing.T) {
	t.Parallel()
	if _, err := Parse("TARGET=value /* first\n\x1b[31m */"); err == nil {
		t.Fatal("terminal controls accepted in annotation")
	}
}

func TestParseRejectsMalformedComments(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"TARGET=host /* outer /* nested */ */",
		"TARGET=host /* note */ garbage",
		"TARGET=host /* note\n*/ NEXT=other",
		"# note\rbroken\nTARGET=host",
		"TARGET=host /* note\n\x7f */",
	} {
		if _, err := Parse(input); err == nil {
			t.Errorf("accepted malformed comment %q", input)
		}
	}
}
