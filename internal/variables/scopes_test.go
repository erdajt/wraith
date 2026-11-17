package variables

import (
	"path/filepath"
	"testing"
)

func TestConversationBindingsResumeWithoutLeakingIntoOtherChats(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	store := Store{Root: t.TempDir(), Conversation: "0123456789abcdef01234567"}
	for _, scope := range []Scope{Global, Directory, Conversation} {
		state, err := store.Load(directory, scope)
		if err != nil {
			t.Fatal(err)
		}
		text := "TARGET=" + string(scope) + ".example"
		if scope == Conversation {
			text += "\nONLY_HERE=private.example // current chat"
		}
		if _, err := store.Save(directory, scope, state.Revision, text); err != nil {
			t.Fatal(err)
		}
	}
	resumed := Store{Root: store.Root, Conversation: store.Conversation}
	state, err := resumed.Load(directory, Conversation)
	if err != nil || state.Snapshot.values["TARGET"] != "conversation.example" || len(state.Entries) != 2 {
		t.Fatalf("conversation bindings not restored: %+v, %v", state.Entries, err)
	}
	other := Store{Root: store.Root, Conversation: "abcdef0123456789abcdef01"}
	state, err = other.Load(directory, Conversation)
	if err != nil || len(state.Entries) != 0 || state.Snapshot.values["TARGET"] != "directory.example" || state.Snapshot.values["ONLY_HERE"] != "" {
		t.Fatalf("conversation leaked: %+v, %v", state.Snapshot.values, err)
	}
	if got := state.Snapshot.Redact("conversation.example private.example"); got != "wraith::TARGET wraith::ONLY_HERE" {
		t.Fatalf("inactive conversation lost privacy protection: %s", got)
	}
	state, err = other.Load(t.TempDir(), Directory)
	if err != nil || state.Snapshot.values["TARGET"] != "global.example" {
		t.Fatalf("directory leaked: %+v, %v", state.Snapshot.values, err)
	}
}

func TestConversationScopeRejectsMissingOrUnsafeIdentity(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"", "../../outside", "abcdef", "0123456789ABCDEF01234567"} {
		store := Store{Root: filepath.Join(t.TempDir(), "variables"), Conversation: id}
		if _, err := store.Load(t.TempDir(), Conversation); err == nil {
			t.Fatalf("accepted identity %q", id)
		}
		if _, err := store.Edit(t.TempDir(), Conversation); err == nil {
			t.Fatalf("editor accepted identity %q", id)
		}
	}
	store := Store{Root: t.TempDir()}
	if _, err := store.Load(t.TempDir(), Scope("unknown")); err == nil {
		t.Fatal("unknown scope accepted")
	}
}
