package command

import "code.linenisgreat.com/purse-first/libs/dewey/internal/0/interfaces"

type CommandComponentReader interface {
	GetCLIFlags() []string
}

type CommandComponent interface {
	CommandComponentReader
	interfaces.CommandComponentWriter
}
