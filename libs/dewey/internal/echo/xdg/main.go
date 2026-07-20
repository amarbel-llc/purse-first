package xdg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.linenisgreat.com/purse-first/libs/dewey/internal/0/interfaces"
	"code.linenisgreat.com/purse-first/libs/dewey/internal/bravo/errors"
	"code.linenisgreat.com/purse-first/libs/dewey/internal/charlie/env_vars"
	"code.linenisgreat.com/purse-first/libs/dewey/internal/delta/files"
	"code.linenisgreat.com/purse-first/libs/dewey/internal/delta/xdg_defaults"
)

type XDG struct {
	Home env_vars.DirectoryLayoutBaseEnvVar
	Cwd  env_vars.DirectoryLayoutBaseEnvVar

	overridePath string

	UtilityName string

	Data    env_vars.DirectoryLayoutBaseEnvVar
	Config  env_vars.DirectoryLayoutBaseEnvVar
	State   env_vars.DirectoryLayoutBaseEnvVar
	Cache   env_vars.DirectoryLayoutBaseEnvVar
	Runtime env_vars.DirectoryLayoutBaseEnvVar
}

func (xdg XDG) GetDirHome() interfaces.DirectoryLayoutBaseEnvVar { return xdg.Home }

func (xdg XDG) GetDirCwd() interfaces.DirectoryLayoutBaseEnvVar { return xdg.Cwd }

func (xdg XDG) GetDirData() interfaces.DirectoryLayoutBaseEnvVar { return xdg.Data }

func (xdg XDG) GetDirConfig() interfaces.DirectoryLayoutBaseEnvVar { return xdg.Config }

func (xdg XDG) GetDirState() interfaces.DirectoryLayoutBaseEnvVar { return xdg.State }

func (xdg XDG) GetDirCache() interfaces.DirectoryLayoutBaseEnvVar { return xdg.Cache }

func (xdg XDG) GetDirRuntime() interfaces.DirectoryLayoutBaseEnvVar { return xdg.Runtime }

func (xdg XDG) GetXDGEnvVars() []env_vars.DirectoryLayoutBaseEnvVar {
	return []env_vars.DirectoryLayoutBaseEnvVar{
		xdg.Data,
		xdg.Config,
		xdg.State,
		xdg.Cache,
		xdg.Runtime,
	}
}

func (xdg XDG) GetXDGPaths() []string {
	return []string{
		xdg.Data.String(),
		xdg.Config.String(),
		xdg.State.String(),
		xdg.Cache.String(),
		xdg.Runtime.String(),
	}
}

func (xdg XDG) AddToEnvVars(envVars interfaces.EnvVars) {
	initElements := xdg.getInitElements()

	for _, element := range initElements {
		envVars[element.defawlt.Name] = element.actual.ActualValue
	}
}

func (xdg XDG) CloneWithUtilityName(
	name string,
) XDG {
	initArgs := InitArgs{
		Home:        xdg.Home.ActualValue,
		Cwd:         xdg.Cwd.ActualValue,
		UtilityName: name,
	}

	if xdg.overridePath != "" {
		errors.PanicIfError(
			xdg.InitializeOverridden(initArgs, xdg.overridePath),
		)
	} else {
		errors.PanicIfError(xdg.InitializeStandardFromEnv(initArgs))
	}

	return xdg
}

func (xdg XDG) CloneWithOverridePath(overridePath string) XDG {
	initArgs := InitArgs{
		Home:        xdg.Home.ActualValue,
		Cwd:         xdg.Cwd.ActualValue,
		UtilityName: xdg.UtilityName,
	}

	errors.PanicIfError(xdg.InitializeOverridden(initArgs, overridePath))

	return xdg
}

func (xdg XDG) CloneWithoutOverride() XDG {
	xdg.overridePath = ""

	initArgs := InitArgs{
		Home:        xdg.Home.ActualValue,
		Cwd:         xdg.Cwd.ActualValue,
		UtilityName: xdg.UtilityName,
	}

	errors.PanicIfError(xdg.InitializeStandardFromEnv(initArgs))

	return xdg
}

func (xdg XDG) IsOverridden() bool {
	return xdg.overridePath != ""
}

func (xdg *XDG) setInitArgs(initArgs InitArgs) (err error) {
	xdg.Home = xdg_defaults.Home.MakeBaseEnvVar(initArgs.Home)
	xdg.Cwd = xdg_defaults.Cwd.MakeBaseEnvVar(initArgs.Cwd)
	xdg.UtilityName = initArgs.UtilityName

	return err
}

func (initArgs InitArgs) makeCwdXDGOverridePath(base string) string {
	return filepath.Join(
		base,
		fmt.Sprintf(".%s", initArgs.UtilityName),
	)
}

func CeilingEnvVarName(utilityName string) string {
	return fmt.Sprintf(
		"%s_CEILING_DIRECTORIES",
		strings.ToUpper(utilityName),
	)
}

func ParseCeilingDirectories(raw string) []string {
	if raw == "" {
		return nil
	}

	var ceilings []string

	for _, entry := range filepath.SplitList(raw) {
		if !filepath.IsAbs(entry) {
			continue
		}

		ceilings = append(ceilings, filepath.Clean(entry))
	}

	return ceilings
}

// IsAboveCeiling reports whether dir is strictly an ancestor of any
// ceiling entry. Both dir and each ceiling are symlink-resolved before
// comparison, matching git's documented GIT_CEILING_DIRECTORIES contract.
// Entries that fail to resolve (e.g. don't exist on disk) fall back to
// filepath.Clean so the ceiling still bounds the walk by name.
func IsAboveCeiling(dir string, ceilings []string) bool {
	resolvedDir := resolveOrClean(dir)

	for _, ceiling := range ceilings {
		if isParentOf(resolvedDir, resolveOrClean(ceiling)) {
			return true
		}
	}

	return false
}

func IsAtOrAboveCeiling(dir string, ceilings []string) bool {
	resolvedDir := resolveOrClean(dir)

	for _, ceiling := range ceilings {
		if resolveOrClean(ceiling) == resolvedDir {
			return true
		}
	}

	return IsAboveCeiling(dir, ceilings)
}

func resolveOrClean(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}

	return resolved
}

func isParentOf(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}

	return len(rel) > 0 && rel[0] != '.'
}

func (initArgs InitArgs) parseCeilingDirectories() []string {
	return ParseCeilingDirectories(
		os.Getenv(CeilingEnvVarName(initArgs.UtilityName)),
	)
}

func (initArgs InitArgs) getCwdXDGOverridePath() (string, bool) {
	dir := initArgs.Cwd
	ceilings := initArgs.parseCeilingDirectories()

	var count int
	for {
		pathCwdXDGOverride := initArgs.makeCwdXDGOverridePath(dir)

		if files.Exists(pathCwdXDGOverride) {
			return dir, true
		}

		if dir == string(filepath.Separator) {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent

		if IsAboveCeiling(dir, ceilings) {
			break
		}

		count++
		if count > 100 {
			panic("too deep")
		}
	}

	return dir, false
}

func (xdg *XDG) InitializeOverriddenIfNecessary(
	initArgs InitArgs,
) (err error) {
	dirCwdXDGOverride, exists := initArgs.getCwdXDGOverridePath()

	if exists {
		if err = xdg.InitializeOverridden(initArgs, dirCwdXDGOverride); err != nil {
			err = errors.Wrap(err)
			return err
		}
	} else {
		if err = xdg.InitializeStandardFromEnv(initArgs); err != nil {
			err = errors.Wrap(err)
			return err
		}
	}

	return err
}

func (xdg *XDG) InitializeOverridden(
	initArgs InitArgs,
	overridePath string,
) (err error) {
	xdg.overridePath = overridePath

	if err = xdg.setInitArgs(initArgs); err != nil {
		err = errors.Wrap(err)
		return err
	}

	getenv := xdg_defaults.MakeGetenv(
		os.Getenv,
		overridePath,
		xdg.UtilityName,
	)

	for _, initElement := range xdg.getInitElements() {
		initElement.actual.Name = initElement.defawlt.Name
		initElement.actual.DefaultValueTemplate = initElement.defawlt.TemplateOverride

		if err = initElement.actual.InitializeXDGTemplate(getenv); err != nil {
			err = errors.Wrap(err)
			return err
		}
	}

	return err
}

func (xdg *XDG) InitializeStandardFromEnv(
	initArgs InitArgs,
) (err error) {
	if err = xdg.setInitArgs(initArgs); err != nil {
		err = errors.Wrap(err)
		return err
	}

	getenv := xdg_defaults.MakeGetenv(
		os.Getenv,
		xdg.Cwd.ActualValue,
		xdg.UtilityName,
	)

	for _, initElement := range xdg.getInitElements() {
		initElement.actual.Name = initElement.defawlt.Name
		initElement.actual.DefaultValueTemplate = initElement.defawlt.TemplateDefault

		if err = initElement.actual.InitializeXDGEnvVarOrTemplate(
			xdg.UtilityName,
			getenv,
		); err != nil {
			err = errors.Wrap(err)
			return err
		}
	}

	return err
}
