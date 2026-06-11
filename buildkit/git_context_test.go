package buildkit

import (
	"context"
	"strings"
	"testing"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
)

// sourceIdentifiers marshals a state and returns the identifiers of all
// source ops in its definition.
func sourceIdentifiers(t *testing.T, st *llb.State) []string {
	t.Helper()

	def, err := st.Marshal(context.Background())
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}

	var identifiers []string
	for _, dt := range def.Def {
		var op pb.Op
		if err := op.UnmarshalVT(dt); err != nil {
			t.Fatalf("failed to unmarshal op: %v", err)
		}
		if src := op.GetSource(); src != nil {
			identifiers = append(identifiers, src.Identifier)
		}
	}
	return identifiers
}

func TestResolveContextStateGitURL(t *testing.T) {
	tests := []struct {
		name           string
		context        string
		wantIdentifier string
	}{
		{
			name:           "https url with commit",
			context:        "https://github.com/org/repo.git#0123456789abcdef0123456789abcdef01234567",
			wantIdentifier: "git://github.com/org/repo.git#0123456789abcdef0123456789abcdef01234567",
		},
		{
			name:           "https url with ref and subdir",
			context:        "https://github.com/org/repo.git#main:apps/api",
			wantIdentifier: "git://github.com/org/repo.git#main",
		},
		{
			name:           "pull request ref",
			context:        "https://github.com/org/repo.git#refs/pull/42/head",
			wantIdentifier: "git://github.com/org/repo.git#refs/pull/42/head",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := resolveContextState(map[string]string{"context": tt.context}, "session-id")
			if err != nil {
				t.Fatalf("resolveContextState(%q) returned error: %v", tt.context, err)
			}

			identifiers := sourceIdentifiers(t, st)
			if len(identifiers) == 0 {
				t.Fatal("no source ops in definition")
			}
			for _, id := range identifiers {
				if strings.HasPrefix(id, "local://") {
					t.Errorf("git context must not produce a local source, got %q", id)
				}
			}
			if !strings.HasPrefix(identifiers[0], tt.wantIdentifier) {
				t.Errorf("source identifier = %q, want prefix %q", identifiers[0], tt.wantIdentifier)
			}
		})
	}
}

func TestResolveContextStateLocalFallback(t *testing.T) {
	// No context option and non-git values both sync from the session.
	for _, contextOpt := range []string{"", "some-local-dir"} {
		t.Run("context="+contextOpt, func(t *testing.T) {
			opts := map[string]string{}
			if contextOpt != "" {
				opts["context"] = contextOpt
			}

			st, err := resolveContextState(opts, "session-id")
			if err != nil {
				t.Fatalf("resolveContextState returned error: %v", err)
			}

			identifiers := sourceIdentifiers(t, st)
			if len(identifiers) != 1 {
				t.Fatalf("got %d source ops, want 1", len(identifiers))
			}
			if identifiers[0] != "local://context" {
				t.Errorf("source identifier = %q, want %q", identifiers[0], "local://context")
			}
		})
	}
}
