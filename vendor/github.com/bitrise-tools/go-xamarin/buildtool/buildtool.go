package buildtool

// PrintableCommand ...
type PrintableCommand interface {
	PrintableCommand() string
}

// RunnableCommand ...
type RunnableCommand interface {
	PrintableCommand() string
	AppendOptions(options []string)
	Run() error
}

// EditableCommand ...
type EditableCommand interface {
	AppendOptions(options []string)
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
