package xbuild

import (
	"fmt"
	"os"

	"github.com/bitrise-io/go-utils/cmdex"
	"github.com/bitrise-tools/go-xamarin/constants"
)

// Model ...
type Model struct {
	buildTool string

	projectPth    string // can be solution or project path
	configuration string
	platform      string
	target        string

	buildIpa       bool
	archiveOnBuild bool

	customOptions []string
}

// New ...
func New(projectPth string) *Model {
	return &Model{
		projectPth: projectPth,
		buildTool:  constants.XbuildPath,
	}
}

// SetTarget ...
func (xbuild *Model) SetTarget(target string) *Model {
	xbuild.target = target
	return xbuild
}

// SetConfiguration ...
func (xbuild *Model) SetConfiguration(configuration string) *Model {
	xbuild.configuration = configuration
	return xbuild
}

// SetPlatform ...
func (xbuild *Model) SetPlatform(platform string) *Model {
	xbuild.platform = platform
	return xbuild
}

// SetBuildIpa ...
func (xbuild *Model) SetBuildIpa() *Model {
	xbuild.buildIpa = true
	return xbuild
}

// SetArchiveOnBuild ...
func (xbuild *Model) SetArchiveOnBuild() *Model {
	xbuild.archiveOnBuild = true
	return xbuild
}

// AppendOptions ...
func (xbuild *Model) AppendOptions(options []string) {
	xbuild.customOptions = options
}

func (xbuild Model) buildCommandSlice() []string {
	cmdSlice := []string{xbuild.buildTool}

	if xbuild.projectPth != "" {
		cmdSlice = append(cmdSlice, xbuild.projectPth)
	}

	if xbuild.target != "" {
		cmdSlice = append(cmdSlice, fmt.Sprintf("/target:%s", xbuild.target))
	}

	if xbuild.configuration != "" {
		cmdSlice = append(cmdSlice, fmt.Sprintf("/p:Configuration=%s", xbuild.configuration))
	}

	if xbuild.platform != "" {
		cmdSlice = append(cmdSlice, fmt.Sprintf("/p:Platform=%s", xbuild.platform))
	}

	if xbuild.archiveOnBuild {
		cmdSlice = append(cmdSlice, "/p:ArchiveOnBuild=true")
	}

	if xbuild.buildIpa {
		cmdSlice = append(cmdSlice, "/p:BuildIpa=true")
	}

	cmdSlice = append(cmdSlice, xbuild.customOptions...)

	//cmdSlice = append(cmdSlice, "/verbosity:minimal", "/nologo")

	return cmdSlice
}

// PrintableCommand ...
func (xbuild Model) PrintableCommand() string {
	cmdSlice := xbuild.buildCommandSlice()

	return cmdex.PrintableCommandArgs(false, cmdSlice)
}

// Run ...
func (xbuild Model) Run() error {
	cmdSlice := xbuild.buildCommandSlice()

	command, err := cmdex.NewCommandFromSlice(cmdSlice)
	if err != nil {
		return err
	}

	command.SetStdout(os.Stdout)
	command.SetStderr(os.Stderr)

	return command.Run()
}
