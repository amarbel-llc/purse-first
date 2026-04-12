package huh

import (
	"github.com/charmbracelet/huh"
)

// Prompter wraps charmbracelet/huh for interactive CLI prompts.
type Prompter struct{}

func (Prompter) Confirm(msg string) (bool, error) {
	var result bool
	err := huh.NewConfirm().
		Title(msg).
		Value(&result).
		Run()
	return result, err
}

func (Prompter) Select(msg string, options []string) (int, error) {
	var result int
	opts := make([]huh.Option[int], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, i)
	}
	err := huh.NewSelect[int]().
		Title(msg).
		Options(opts...).
		Value(&result).
		Run()
	return result, err
}

func (Prompter) Input(msg string) (string, error) {
	var result string
	err := huh.NewInput().
		Title(msg).
		Value(&result).
		Run()
	return result, err
}
