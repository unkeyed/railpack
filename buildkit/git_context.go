package buildkit

import (
	"strconv"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/pkg/errors"
)

const (
	// localNameContext is the name of the local mount holding the application
	// source when the context is synced from the client session.
	localNameContext = "context"

	// keyContextKeepGitDirArg mirrors the dockerfile frontend's build arg for
	// keeping the .git directory when fetching a git context.
	keyContextKeepGitDirArg = "build-arg:BUILDKIT_CONTEXT_KEEP_GIT_DIR"
)

// resolveContextState returns the LLB state for the application source.
//
// When the "context" frontend option is a git URL (e.g.
// "https://github.com/org/repo.git#ref" or "...#ref:subdir"), the source is
// fetched by BuildKit directly on the build machine, mirroring the behavior
// of the dockerfile frontend. Authentication uses the standard
// GIT_AUTH_TOKEN / GIT_AUTH_HEADER session secrets (optionally suffixed with
// the host, e.g. "GIT_AUTH_TOKEN.github.com"). The .git directory is dropped
// unless BUILDKIT_CONTEXT_KEEP_GIT_DIR is set, matching the dockerfile
// frontend's default.
//
// For any other value (including the common case of no context option at
// all), the context is synced from the client session's "context" local
// mount, which is the historical behavior.
func resolveContextState(opts map[string]string, sessionID string) (*llb.State, error) {
	var keepGit *bool
	if v, err := strconv.ParseBool(opts[keyContextKeepGitDirArg]); err == nil {
		keepGit = &v
	}

	if st, isGit, err := dockerui.DetectGitContext(opts[localNameContext], keepGit); isGit {
		if err != nil {
			return nil, errors.Wrapf(err, "invalid git context %q", opts[localNameContext])
		}
		return st, nil
	}

	st := llb.Local(localNameContext,
		llb.SharedKeyHint("local"),
		llb.SessionID(sessionID),
		llb.WithCustomName("loading ."),
		llb.FollowPaths([]string{"."}),
	)
	return &st, nil
}
