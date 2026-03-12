package packagebrew

import (
	"fmt"
	"strings"
	"unicode"
)

type FormulaOptions struct {
	Name        string
	Description string
	Version     string
	Homepage    string
	License     string
	ReleaseRepo string
	Binary      bool
	Private     bool
	Hashes      map[string]string // platform -> sha256 (empty key for platform-independent)
	BrewDeps    []string
}

type MetaFormulaOptions struct {
	Name        string
	Description string
	Version     string
	License     string
	ReleaseRepo string
	Hash        string
	Packages    []string
	AutoInstall bool
	Private     bool
}

// Platform mapping for Homebrew conditionals.
var platformBlocks = []struct {
	key    string
	os     string
	archFn string
}{
	{"darwin-arm64", "macos", "arm?"},
	{"darwin-amd64", "macos", "intel?"},
	{"linux-arm64", "linux", "arm?"},
	{"linux-amd64", "linux", "intel?"},
}

func toClassName(name string) string {
	var b strings.Builder
	upper := true
	for _, r := range name {
		if r == '-' {
			upper = true
			continue
		}
		if upper {
			b.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func GenerateFormula(opts FormulaOptions) string {
	var b strings.Builder

	if opts.Private {
		b.WriteString("require_relative \"../lib/custom_download_strategy\"\n\n")
	}

	homepage := opts.Homepage
	if homepage == "" {
		homepage = fmt.Sprintf("https://github.com/%s", opts.ReleaseRepo)
	}

	fmt.Fprintf(&b, "class %s < Formula\n", toClassName(opts.Name))
	fmt.Fprintf(&b, "  desc %q\n", opts.Description)
	fmt.Fprintf(&b, "  homepage %q\n", homepage)
	fmt.Fprintf(&b, "  version %q\n", opts.Version)
	fmt.Fprintf(&b, "  license %q\n", opts.License)
	b.WriteString("\n")

	using := ""
	if opts.Private {
		using = ",\n          using: GitHubPrivateRepositoryReleaseDownloadStrategy"
	}

	if opts.Binary {
		// Group platforms by OS.
		type archEntry struct {
			archFn string
			hash   string
			url    string
		}
		osGroups := map[string][]archEntry{}
		for _, pb := range platformBlocks {
			hash, ok := opts.Hashes[pb.key]
			if !ok {
				continue
			}
			url := fmt.Sprintf(
				"https://github.com/%s/releases/download/v%s/%s-%s-%s.tar.gz",
				opts.ReleaseRepo, opts.Version, opts.Name, opts.Version, pb.key,
			)
			osGroups[pb.os] = append(osGroups[pb.os], archEntry{pb.archFn, hash, url})
		}

		for _, osName := range []string{"macos", "linux"} {
			entries, ok := osGroups[osName]
			if !ok {
				continue
			}
			fmt.Fprintf(&b, "  on_%s do\n", osName)
			for i, e := range entries {
				if i == 0 {
					fmt.Fprintf(&b, "    if Hardware::CPU.%s\n", e.archFn)
				} else {
					fmt.Fprintf(&b, "    elsif Hardware::CPU.%s\n", e.archFn)
				}
				fmt.Fprintf(&b, "      url %q%s\n", e.url, using)
				fmt.Fprintf(&b, "      sha256 %q\n", e.hash)
			}
			b.WriteString("    end\n")
			b.WriteString("  end\n\n")
		}
	} else {
		// Skill-only: single URL.
		hash := opts.Hashes[""]
		url := fmt.Sprintf(
			"https://github.com/%s/releases/download/v%s/%s-%s.tar.gz",
			opts.ReleaseRepo, opts.Version, opts.Name, opts.Version,
		)
		fmt.Fprintf(&b, "  url %q%s\n", url, using)
		fmt.Fprintf(&b, "  sha256 %q\n", hash)
		b.WriteString("\n")
	}

	for _, dep := range opts.BrewDeps {
		fmt.Fprintf(&b, "  depends_on %q\n", dep)
	}
	if len(opts.BrewDeps) > 0 {
		b.WriteString("\n")
	}

	b.WriteString("  def install\n")
	if opts.Binary {
		fmt.Fprintf(&b, "    bin.install \"bin/%s\"\n", opts.Name)
	}
	fmt.Fprintf(&b, "    (share/\"purse-first/%s\").install Dir[\"share/purse-first/%s/*\"]\n", opts.Name, opts.Name)
	b.WriteString("  end\n\n")

	b.WriteString("  test do\n")
	if opts.Binary {
		fmt.Fprintf(&b, "    system bin/%q, \"--help\"\n", opts.Name)
	} else {
		fmt.Fprintf(&b, "    assert_predicate share/\"purse-first/%s/.claude-plugin/plugin.json\", :exist?\n", opts.Name)
	}
	b.WriteString("  end\n")
	b.WriteString("end\n")

	return b.String()
}

func GenerateMetaFormula(opts MetaFormulaOptions) string {
	var b strings.Builder

	if opts.Private {
		b.WriteString("require_relative \"../lib/custom_download_strategy\"\n\n")
	}

	fmt.Fprintf(&b, "class %s < Formula\n", toClassName(opts.Name))
	fmt.Fprintf(&b, "  desc %q\n", opts.Description)
	fmt.Fprintf(&b, "  homepage \"https://github.com/%s\"\n", opts.ReleaseRepo)
	fmt.Fprintf(&b, "  version %q\n", opts.Version)
	fmt.Fprintf(&b, "  license %q\n", opts.License)

	url := fmt.Sprintf(
		"https://github.com/%s/releases/download/v%s/%s-%s.tar.gz",
		opts.ReleaseRepo, opts.Version, opts.Name, opts.Version,
	)
	using := ""
	if opts.Private {
		using = ",\n      using: GitHubPrivateRepositoryReleaseDownloadStrategy"
	}
	fmt.Fprintf(&b, "  url %q%s\n", url, using)
	fmt.Fprintf(&b, "  sha256 %q\n", opts.Hash)
	b.WriteString("\n")

	for _, pkg := range opts.Packages {
		fmt.Fprintf(&b, "  depends_on %q\n", pkg)
	}
	b.WriteString("\n")

	b.WriteString("  def install\n")
	b.WriteString("    (share/\"purse-first\").install \"marketplace.json\"\n")
	b.WriteString("    (prefix/\".claude-plugin\").install share/\"purse-first/marketplace.json\"\n")
	b.WriteString("  end\n\n")

	if opts.AutoInstall {
		b.WriteString("  def post_install\n")
		b.WriteString("    system \"purse-first\", \"install\", prefix\n")
		b.WriteString("  end\n\n")
	}

	b.WriteString("  test do\n")
	b.WriteString("    assert_predicate prefix/\".claude-plugin/marketplace.json\", :exist?\n")
	b.WriteString("  end\n")
	b.WriteString("end\n")

	return b.String()
}
