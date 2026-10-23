package sourcepack

import (
	"bytes"
	"context"
	"io/fs"

	"github.com/priyanshjhaa/Sprout/backend/internal/nodeapp"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
	"github.com/priyanshjhaa/Sprout/backend/internal/staticsite"
)

// Check runs the server's source stage on a packed archive, on this machine:
// the same extractor and the same static or Node.js contract, in the same order
// as buildjob. It returns the server's own errors (source.ErrInvalid,
// staticsite.ErrContract, nodeapp.ErrDependencies, a *nodeapp.ContractError
// and so on), so a client can say what will fail before uploading. Nothing is
// built or run.
func Check(ctx context.Context, archive Archive) error {
	_, err := source.WithArchive(ctx, bytes.NewReader(archive.Tar), func(ctx context.Context, files fs.FS) error {
		if _, err := fs.Stat(files, "package.json"); err != nil {
			return staticsite.Validate(ctx, files)
		}
		if err := nodeapp.CheckPackages(files); err != nil {
			return err
		}
		return nodeapp.Validate(ctx, files)
	})
	return err
}
