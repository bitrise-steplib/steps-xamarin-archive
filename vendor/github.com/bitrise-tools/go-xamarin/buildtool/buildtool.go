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
