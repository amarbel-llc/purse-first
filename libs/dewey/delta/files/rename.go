package files

import (
	"os"

	"github.com/amarbel-llc/purse-first/libs/dewey/bravo/errors"
)

func Rename(src, dst string) (err error) {
	if err = os.Rename(src, dst); err != nil {
		err = errors.Wrapf(err, "Src: %q, Dst: %q", src, dst)
		return err
	}

	return err
}
