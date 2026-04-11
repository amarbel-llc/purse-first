package xdg

import (
	"github.com/amarbel-llc/purse-first/libs/dewey/charlie/env_vars"
	"github.com/amarbel-llc/purse-first/libs/dewey/delta/xdg_defaults"
)

type initElement struct {
	defawlt xdg_defaults.DefaultEnvVar
	actual  *env_vars.DirectoryLayoutBaseEnvVar
}

func (xdg *XDG) getInitElements() []initElement {
	return []initElement{
		{
			defawlt: xdg_defaults.Data,
			actual:  &xdg.Data,
		},
		{
			defawlt: xdg_defaults.Config,
			actual:  &xdg.Config,
		},
		{
			defawlt: xdg_defaults.State,
			actual:  &xdg.State,
		},
		{
			defawlt: xdg_defaults.Cache,
			actual:  &xdg.Cache,
		},
		{
			defawlt: xdg_defaults.Runtime,
			actual:  &xdg.Runtime,
		},
	}
}
