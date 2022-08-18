package binutils

import (
	"io"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func CopyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Chmod(constants.DefaultPerms755); err != nil {
		return err
	}
	return nil
}
