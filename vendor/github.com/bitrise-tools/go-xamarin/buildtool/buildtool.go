package buildtool

// PrintableCommand ...
type PrintableCommand interface {
	PrintableCommand() string
}

// RunnableCommand ...
type RunnableCommand interface {
	PrintableCommand() string
	Run() error
}
