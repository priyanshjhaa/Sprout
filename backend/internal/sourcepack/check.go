package sourcepack

import (
	"bytes"
	"context"
	"io/fs"

	"github.com/priyanshjhaa/Sprout/backend/internal/nodeapp"
	"github.com/priyanshjhaa/Sprout/backend/internal/pythonapp"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
	"github.com/priyanshjhaa/Sprout/backend/internal/staticsite"
)

// Check runs the server's source stage on a packed archive, on this machine:
// the same extractor and the same static, Node.js or Python contract, in the
// same order as buildjob. It returns the server's own errors
// (source.ErrInvalid, staticsite.ErrContract, a *nodeapp.ContractError, a
// *pythonapp.ContractError and so on), so a client can say what will fail
// before uploading. Nothing is built or run.
func Check(ctx context.Context, archive Archive) error {
	_, err := source.WithArchive(ctx, bytes.NewReader(archive.Tar), func(ctx context.Context, files fs.FS) error {
		if _, err := fs.Stat(files, "package.json"); err == nil {
			return nodeapp.Validate(ctx, files)
		}
		if pythonapp.IsPython(files) {
			return pythonapp.Validate(ctx, files)
		}
		return staticsite.Validate(ctx, files)
	})
	return err
}
