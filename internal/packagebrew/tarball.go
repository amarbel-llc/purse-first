package packagebrew

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type TarballOptions struct {
	Name      string
	Version   string
	Platform  string
	BinPath   string
	ShareDir  string
	OutputDir string
}

type MarketplaceTarballOptions struct {
	Name            string
	Version         string
	MarketplaceJSON []byte
	OutputDir       string
}

func tarballFilename(name, version, platform string) string {
	if platform == "" {
		return fmt.Sprintf("%s-%s.tar.gz", name, version)
	}
	return fmt.Sprintf("%s-%s-%s.tar.gz", name, version, platform)
}

func CreateTarball(opts TarballOptions) (outPath string, retErr error) {
	filename := tarballFilename(opts.Name, opts.Version, opts.Platform)
	outPath = filepath.Join(opts.OutputDir, filename)

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("creating tarball: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer func() {
		if cerr := gw.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("finalizing gzip: %w", cerr)
		}
	}()

	tw := tar.NewWriter(gw)
	defer func() {
		if cerr := tw.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("finalizing tar: %w", cerr)
		}
	}()

	if opts.BinPath != "" {
		if err := addFileToTar(tw, opts.BinPath, filepath.Join("bin", opts.Name)); err != nil {
			return "", fmt.Errorf("adding binary: %w", err)
		}
	}

	if opts.ShareDir != "" {
		shareParent := filepath.Dir(filepath.Dir(filepath.Dir(opts.ShareDir)))
		if err := addDirToTar(tw, opts.ShareDir, shareParent); err != nil {
			return "", fmt.Errorf("adding share dir: %w", err)
		}
	}

	return outPath, nil
}

func CreateMarketplaceTarball(opts MarketplaceTarballOptions) (outPath string, retErr error) {
	filename := tarballFilename(opts.Name, opts.Version, "")
	outPath = filepath.Join(opts.OutputDir, filename)

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("creating marketplace tarball: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer func() {
		if cerr := gw.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("finalizing gzip: %w", cerr)
		}
	}()

	tw := tar.NewWriter(gw)
	defer func() {
		if cerr := tw.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("finalizing tar: %w", cerr)
		}
	}()

	hdr := &tar.Header{
		Name: "marketplace.json",
		Mode: 0o644,
		Size: int64(len(opts.MarketplaceJSON)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return "", err
	}
	if _, err := tw.Write(opts.MarketplaceJSON); err != nil {
		return "", err
	}

	return outPath, nil
}

func addFileToTar(tw *tar.Writer, srcPath, tarPath string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name: tarPath,
		Mode: int64(info.Mode()),
		Size: info.Size(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}

func addDirToTar(tw *tar.Writer, dirPath, baseDir string) error {
	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		rel = strings.ReplaceAll(rel, string(filepath.Separator), "/")

		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			hdr := &tar.Header{
				Name:     rel + "/",
				Mode:     int64(info.Mode()),
				Typeflag: tar.TypeDir,
			}
			return tw.WriteHeader(hdr)
		}

		return addFileToTar(tw, path, rel)
	})
}
