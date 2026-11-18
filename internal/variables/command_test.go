package variables

import (
	"strings"
	"testing"
)

func TestCommandReferencesCatchBarePrivateArguments(t *testing.T) {
	t.Parallel()
	s := NewSnapshot([]Entry{{Name: "TARGET_DOMAIN", Value: "private.example"}}, nil)

	for _, command := range []string{
		"dig +short TARGET_DOMAIN | xargs -I {} printf '%s' {}",
		"env -u TARGET_DOMAIN dig TARGET_DOMAIN",
		"env -C TARGET_DOMAIN printf done",
		"unset TARGET_DOMAIN; dig TARGET_DOMAIN",
		"curl 'TARGET_DOMAIN'",
		`curl "TARGET_DOMAIN"`,
		"target=TARGET_DOMAIN; printf '%s' \"$target\"",
		"printf saved >TARGET_DOMAIN",
	} {
		if err := s.CheckCommand(command); err == nil || !strings.Contains(err.Error(), "wraith::TARGET_DOMAIN") || strings.Contains(err.Error(), "private.example") {
			t.Errorf("missing private correction for %q: %v", command, err)
		}
	}
}

func TestCommandReferencesKeepShellSyntaxAndUnrelatedNames(t *testing.T) {
	t.Parallel()
	s := NewSnapshot([]Entry{{Name: "TARGET_DOMAIN", Value: "private.example"}}, nil)

	for _, command := range []string{
		"dig +short wraith::TARGET_DOMAIN",
		"unset TARGET_DOMAIN",
		"builtin unset TARGET_DOMAIN",
		"printenv TARGET_DOMAIN",
		"command printenv TARGET_DOMAIN",
		"/usr/bin/printenv 'TARGET_DOMAIN'",
		"env -u TARGET_DOMAIN printf done",
		"env --unset TARGET_DOMAIN printf done",
		"env --unset=TARGET_DOMAIN printf done",
		"export TARGET_DOMAIN",
		`curl "$TARGET_DOMAIN"`,
		`curl "${TARGET_DOMAIN}"`,
		"TARGET_DOMAIN=host.test; curl \"$TARGET_DOMAIN\"",
		"printf '%s' TARGET_DOMAIN_BACKUP",
		"printf '%s' NOT_CONFIGURED",
		"printf done # TARGET_DOMAIN",
		"cat <<EOF\nTARGET_DOMAIN\nEOF",
		"printf 'TARGET_DOMAIN is the variable name'",
		"printf 'incomplete",
	} {
		if err := s.CheckCommand(command); err != nil {
			t.Errorf("changed unrelated shell syntax %q: %v", command, err)
		}
	}
}
