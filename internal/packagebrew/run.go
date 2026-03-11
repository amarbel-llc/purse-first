package packagebrew

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/amarbel-llc/purse-first/internal/marketplace"
)

type RunOptions struct {
	ConfigPath  string
	OutputDir   string
	AutoInstall bool
}

func Run(opts RunOptions) error {
	cfg, err := ReadConfig(opts.ConfigPath)
	if err != nil {
		return err
	}

	formulaDir := filepath.Join(opts.OutputDir, "Formula")
	tarballDir := filepath.Join(opts.OutputDir, "tarballs")
	if err := os.MkdirAll(formulaDir, 0o755); err != nil {
		return fmt.Errorf("creating Formula dir: %w", err)
	}
	if err := os.MkdirAll(tarballDir, 0o755); err != nil {
		return fmt.Errorf("creating tarballs dir: %w", err)
	}

	// Determine a consistent version from the first package (or use "0.0.0").
	metaVersion := "0.0.0"

	// Sort package names for deterministic output.
	pkgNames := make([]string, 0, len(cfg.Packages))
	for name := range cfg.Packages {
		pkgNames = append(pkgNames, name)
	}
	sort.Strings(pkgNames)

	if len(pkgNames) > 0 {
		metaVersion = cfg.Packages[pkgNames[0]].Version
	}

	// 1. Create tarballs and collect hashes.
	type pkgHashes struct {
		hashes map[string]string // platform -> sha256
	}
	allHashes := make(map[string]pkgHashes)

	for _, name := range pkgNames {
		pkg := cfg.Packages[name]
		hashes := make(map[string]string)

		if pkg.Binary {
			for platform, binPath := range pkg.Platforms {
				tarPath, err := CreateTarball(TarballOptions{
					Name:      name,
					Version:   pkg.Version,
					Platform:  platform,
					BinPath:   binPath,
					ShareDir:  pkg.Share,
					OutputDir: tarballDir,
				})
				if err != nil {
					return fmt.Errorf("creating tarball for %s/%s: %w", name, platform, err)
				}

				hash, err := sha256File(tarPath)
				if err != nil {
					return fmt.Errorf("hashing tarball %s: %w", tarPath, err)
				}
				hashes[platform] = hash
			}
		} else {
			tarPath, err := CreateTarball(TarballOptions{
				Name:      name,
				Version:   pkg.Version,
				ShareDir:  pkg.Share,
				OutputDir: tarballDir,
			})
			if err != nil {
				return fmt.Errorf("creating tarball for %s: %w", name, err)
			}

			hash, err := sha256File(tarPath)
			if err != nil {
				return fmt.Errorf("hashing tarball %s: %w", tarPath, err)
			}
			hashes[""] = hash
		}

		allHashes[name] = pkgHashes{hashes: hashes}
	}

	// 2. Generate marketplace.json by discovering plugins from share dirs.
	mpConfig := marketplace.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
		Repo:        cfg.ReleaseRepo,
		Owner: marketplace.Owner{
			Name:  cfg.Owner.Name,
			Email: cfg.Owner.Email,
		},
		Plugins: make(map[string]marketplace.PluginMeta),
	}

	var discovered []marketplace.DiscoveredPlugin
	for _, name := range pkgNames {
		pkg := cfg.Packages[name]

		mpConfig.Plugins[name] = marketplace.PluginMeta{
			Description: pkg.Description,
			Version:     pkg.Version,
			Homepage:    pkg.Homepage,
			Category:    pkg.Category,
			Tags:        pkg.Tags,
		}

		// Discover MCP servers and skills from the share directory.
		sharePkgs, err := marketplace.DiscoverPlugins(filepath.Dir(pkg.Share))
		if err != nil {
			return fmt.Errorf("discovering plugins in %s: %w", pkg.Share, err)
		}

		for _, dp := range sharePkgs {
			if dp.Name == name {
				discovered = append(discovered, dp)
				break
			}
		}
	}

	mp := marketplace.Generate(mpConfig, discovered)
	mpJSON, err := json.MarshalIndent(mp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling marketplace.json: %w", err)
	}
	mpJSON = append(mpJSON, '\n')

	mpDir := filepath.Join(opts.OutputDir, ".claude-plugin")
	if err := os.MkdirAll(mpDir, 0o755); err != nil {
		return fmt.Errorf("creating .claude-plugin dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(mpDir, "marketplace.json"), mpJSON, 0o644); err != nil {
		return fmt.Errorf("writing marketplace.json: %w", err)
	}

	// 3. Create meta-formula tarball.
	metaTarPath, err := CreateMarketplaceTarball(MarketplaceTarballOptions{
		Name:            cfg.Name,
		Version:         metaVersion,
		MarketplaceJSON: mpJSON,
		OutputDir:       tarballDir,
	})
	if err != nil {
		return fmt.Errorf("creating meta tarball: %w", err)
	}
	metaHash, err := sha256File(metaTarPath)
	if err != nil {
		return fmt.Errorf("hashing meta tarball: %w", err)
	}

	// 4. Generate per-package formulas.
	for _, name := range pkgNames {
		pkg := cfg.Packages[name]

		formula := GenerateFormula(FormulaOptions{
			Name:        name,
			Description: pkg.Description,
			Version:     pkg.Version,
			Homepage:    pkg.Homepage,
			License:     cfg.License,
			ReleaseRepo: cfg.ReleaseRepo,
			Binary:      pkg.Binary,
			Hashes:      allHashes[name].hashes,
			BrewDeps:    pkg.BrewDeps,
		})

		formulaPath := filepath.Join(formulaDir, name+".rb")
		if err := os.WriteFile(formulaPath, []byte(formula), 0o644); err != nil {
			return fmt.Errorf("writing formula %s: %w", name, err)
		}
	}

	// 5. Generate meta-formula.
	metaFormula := GenerateMetaFormula(MetaFormulaOptions{
		Name:        cfg.Name,
		Description: cfg.Description,
		Version:     metaVersion,
		License:     cfg.License,
		ReleaseRepo: cfg.ReleaseRepo,
		Hash:        metaHash,
		Packages:    pkgNames,
		AutoInstall: opts.AutoInstall,
	})
	metaFormulaPath := filepath.Join(formulaDir, cfg.Name+".rb")
	if err := os.WriteFile(metaFormulaPath, []byte(metaFormula), 0o644); err != nil {
		return fmt.Errorf("writing meta formula: %w", err)
	}

	// 6. Generate README.
	var readmePkgs []ReadmePackage
	for _, name := range pkgNames {
		readmePkgs = append(readmePkgs, ReadmePackage{
			Name:        name,
			Description: cfg.Packages[name].Description,
		})
	}

	readme := GenerateReadme(ReadmeOptions{
		TapName:         cfg.TapName,
		Description:     cfg.Description,
		Packages:        readmePkgs,
		MetaFormulaName: cfg.Name,
	})
	if err := os.WriteFile(filepath.Join(opts.OutputDir, "README.md"), []byte(readme), 0o644); err != nil {
		return fmt.Errorf("writing README: %w", err)
	}

	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
