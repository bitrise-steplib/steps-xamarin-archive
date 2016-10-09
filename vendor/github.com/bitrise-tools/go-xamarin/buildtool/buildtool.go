package buildtool

// RunnableCommand ...
type RunnableCommand interface {
	PrintableCommand() string
	SetCustomOptions(options ...string)
	Run() error
}

// PrintableCommand ...
type PrintableCommand interface {
	PrintableCommand() string
}

// EditableCommand ...
type EditableCommand interface {
	SetCustomOptions(options ...string)
}

// BuildCommandSliceContains ...
func BuildCommandSliceContains(cmdSlice []RunnableCommand, cmd RunnableCommand) bool {
	for _, c := range cmdSlice {
		if c.PrintableCommand() == cmd.PrintableCommand() {
			return true
		}
	}
	return false
}
