package gwext

import (
	"strings"
	"testing"
)

func TestParseStringsFlag(t *testing.T) {
	decls := []Flag{Strs("tag", "build tags").Alias("t")}
	cases := []struct {
		argv []string
		want string // comma-joined
	}{
		{[]string{"--tag", "a", "--tag", "b"}, "a,b"},
		{[]string{"--tag", "a,b,c"}, "a,b,c"},
		{[]string{"--tag", "x", "-t", "y,z"}, "x,y,z"},
		{[]string{}, ""},
	}
	for _, tc := range cases {
		c := &Context{}
		c.flags, _ = parseFlags("tagit", decls, tc.argv)
		if got := strings.Join(c.Strings("tag"), ","); got != tc.want {
			t.Errorf("%v: tags=%q want %q", tc.argv, got, tc.want)
		}
	}
}

func TestParseFlagAliases(t *testing.T) {
	decls := []Flag{
		Str("name", "world", "who").Alias("n"),
		Int("count", 1, "n").Alias("c", "num"),
		Bool("loud", "shout").Alias("l"),
	}

	cases := []struct {
		argv []string
		name string
		cnt  int
		loud bool
	}{
		{[]string{"--name", "Alice", "--count", "3", "--loud"}, "Alice", 3, true},
		{[]string{"-n", "Bob", "-c", "5", "-l"}, "Bob", 5, true},
		{[]string{"--num=9"}, "world", 9, false},
		{[]string{"-name=Carol"}, "Carol", 1, false},
	}
	for _, tc := range cases {
		c := &Context{}
		c.flags, _ = parseFlags("greet", decls, tc.argv)
		if got := c.String("name"); got != tc.name {
			t.Errorf("%v: name=%q want %q", tc.argv, got, tc.name)
		}
		if got := c.Int("count"); got != tc.cnt {
			t.Errorf("%v: count=%d want %d", tc.argv, got, tc.cnt)
		}
		if got := c.Bool("loud"); got != tc.loud {
			t.Errorf("%v: loud=%v want %v", tc.argv, got, tc.loud)
		}
	}
}
