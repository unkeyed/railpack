package buildkit

import (
	"strconv"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/pkg/errors"
	p "github.com/railwayapp/railpack/core/plan"
)

const (
	// localNameContext is the name of the local mount holding the application
	// source when the context is synced from the client session.
	localNameContext = "context"

	// keyContextKeepGitDirArg mirrors the dockerfile frontend's build arg for
	// keeping the .git directory when fetching a git context.
	keyContextKeepGitDirArg = "build-arg:BUILDKIT_CONTEXT_KEEP_GIT_DIR"
)

// resolveContextState returns a git source state when the "context" frontend
// option is a git URL (e.g. "https://github.com/org/repo.git#ref" or
// "...#ref:subdir"), so BuildKit fetches the repository directly on the build
// machine, mirroring the dockerfile frontend. Authentication uses the standard
// GIT_AUTH_TOKEN / GIT_AUTH_HEADER session secrets (optionally suffixed with
// the host, e.g. "GIT_AUTH_TOKEN.github.com"). The .git directory is dropped
// unless BUILDKIT_CONTEXT_KEEP_GIT_DIR is set, matching the dockerfile
// frontend's default.
//
// For any other value (including no context option at all) it returns nil, and
// ConvertPlanToLLB syncs the context from the client session's local mount.
func resolveContextState(opts map[string]string) (*llb.State, error) {
	var keepGit *bool
	if v, err := strconv.ParseBool(opts[keyContextKeepGitDirArg]); err == nil {
		keepGit = &v
	}

	st, isGit, err := dockerui.DetectGitContext(opts[localNameContext], keepGit)
	if !isGit {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "invalid git context %q", opts[localNameContext])
	}
	return st, nil
}

// resolveSourceState returns the application source for the build graph with
// the plan's exclude patterns applied, whether the source is the session's
// local mount or a caller-provided state such as a git checkout.
func resolveSourceState(plan *p.BuildPlan, opts ConvertPlanOptions) (*llb.State, error) {
	if opts.ContextState == nil {
		// by default, the whole directory is transferred into context, we don't need to explicitly include it
		localOpts := []llb.LocalOption{
			llb.SharedKeyHint("local"),
			llb.SessionID(opts.SessionID),
			llb.WithCustomName("loading ."),
		}

		// note that exclude patterns can contain inverse (inclusions) patterns. The llb.IncludePatterns should *not* be used for this
		if len(plan.Exclude) > 0 {
			localOpts = append(localOpts, llb.ExcludePatterns(plan.Exclude))
		}

		st := llb.Local(localNameContext, localOpts...)
		return &st, nil
	}

	if len(plan.Exclude) == 0 {
		return opts.ContextState, nil
	}

	st := llb.Scratch().File(
		llb.Copy(*opts.ContextState, "/", "/", &llb.CopyInfo{
			CopyDirContentsOnly: true,
			ExcludePatterns:     plan.Exclude,
		}),
		llb.WithCustomName("filtering context"),
	)
	return &st, nil
}
