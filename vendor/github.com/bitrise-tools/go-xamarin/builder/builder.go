package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-xamarin/buildtool"
	"github.com/bitrise-tools/go-xamarin/buildtool/mdtool"
	"github.com/bitrise-tools/go-xamarin/buildtool/xbuild"
	"github.com/bitrise-tools/go-xamarin/constants"
	"github.com/bitrise-tools/go-xamarin/project"
	"github.com/bitrise-tools/go-xamarin/solution"
	"github.com/bitrise-tools/go-xamarin/utility"
)

// Model ...
type Model struct {
	solution solution.Model

	projectTypeWhitelist []constants.ProjectType
	forceMDTool          bool
}

// OutputMap ...
type OutputMap map[constants.ProjectType]map[constants.OutputType]string

// BuildCommandCallback ...
type BuildCommandCallback func(project project.Model, command buildtool.PrintableCommand)

// ClearCommandCallback ...
type ClearCommandCallback func(project project.Model, dir string)

// New ...
func New(solutionPth string, projectTypeWhitelist []constants.ProjectType, forceMDTool bool) (Model, error) {
	if err := validateSolutionPth(solutionPth); err != nil {
		return Model{}, err
	}

	solution, err := solution.New(solutionPth, true)
	if err != nil {
		return Model{}, err
	}

	if projectTypeWhitelist == nil {
		projectTypeWhitelist = []constants.ProjectType{}
	}

	return Model{
		solution: solution,

		projectTypeWhitelist: projectTypeWhitelist,
		forceMDTool:          forceMDTool,
	}, nil
}

func (builder Model) filteredProjects() []project.Model {
	projects := []project.Model{}

	for _, proj := range builder.solution.ProjectMap {
		if !isProjectTypeAllowed(proj.ProjectType, builder.projectTypeWhitelist...) {
			continue
		}

		if proj.ProjectType != constants.ProjectTypeUnknown {
			projects = append(projects, proj)
		}
	}

	return projects
}

func (builder Model) buildableProjects(configuration, platform string) []project.Model {
	projects := []project.Model{}

	solutionConfig := utility.ToConfig(configuration, platform)
	filteredProjects := builder.filteredProjects()

	for _, proj := range filteredProjects {
		//
		// Solution config - project config mapping
		_, ok := proj.ConfigMap[solutionConfig]
		if !ok {
			// fmt.Sprintf("project (%s) do not have config for solution config (%s), skipping...", proj.Name, solutionConfig)
			continue
		}

		if (proj.ProjectType == constants.ProjectTypeIos ||
			proj.ProjectType == constants.ProjectTypeMac ||
			proj.ProjectType == constants.ProjectTypeTVOs) &&
			proj.OutputType != "exe" {
			// fmt.Sprintf("project (%s) does not archivable based on output type (%s), skipping...", project.Name, project.OutputType)
			continue
		}
		if proj.ProjectType == constants.ProjectTypeAndroid &&
			!proj.AndroidApplication {
			// fmt.Sprintf("(%s) is not an android application project, skipping...", proj.Name)
			continue
		}

		if proj.ProjectType != constants.ProjectTypeUnknown {
			projects = append(projects, proj)
		}
	}

	return projects
}

// CleanAll ...
func (builder Model) CleanAll(callback ClearCommandCallback) error {
	filteredProjects := builder.filteredProjects()
	for _, proj := range filteredProjects {

		projectDir := filepath.Dir(proj.Pth)

		{
			binPth := filepath.Join(projectDir, "bin")
			if exist, err := pathutil.IsDirExists(binPth); err != nil {
				return err
			} else if exist {
				if callback != nil {
					callback(proj, binPth)
				}

				if err := os.RemoveAll(binPth); err != nil {
					return err
				}
			}
		}

		{
			objPth := filepath.Join(projectDir, "obj")
			if exist, err := pathutil.IsDirExists(objPth); err != nil {
				return err
			} else if exist {
				if callback != nil {
					callback(proj, objPth)
				}

				if err := os.RemoveAll(objPth); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// BuildAllProjects ...
func (builder Model) BuildAllProjects(configuration, platform string, callback BuildCommandCallback) ([]string, error) {
	warnings := []string{}
	buildableProjects := builder.buildableProjects(configuration, platform)
	solutionConfig := utility.ToConfig(configuration, platform)

	for _, proj := range buildableProjects {
		projectConfigKey, ok := proj.ConfigMap[solutionConfig]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("project (%s) do not have config for solution config (%s), skipping...", proj.Name, solutionConfig))
			continue
		}

		projectConfig, ok := proj.Configs[projectConfigKey]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("project (%s) contains mapping for solution config (%s), but does not have project configuration", proj.Name, solutionConfig))
			continue
		}

		// Prepare build commands
		buildCommands := []buildtool.RunnableCommand{}

		switch proj.ProjectType {
		case constants.ProjectTypeIos, constants.ProjectTypeTVOs:
			if builder.forceMDTool {
				command := mdtool.New(builder.solution.Pth).SetTarget("build")
				command.SetConfiguration(projectConfig.Configuration)
				command.SetPlatform(projectConfig.Platform)
				command.SetProjectName(proj.Name)

				buildCommands = append(buildCommands, command)

				if isArchitectureArchiveable(projectConfig.MtouchArchs) {
					command := mdtool.New(builder.solution.Pth).SetTarget("archive")
					command.SetConfiguration(projectConfig.Configuration)
					command.SetPlatform(projectConfig.Platform)
					command.SetProjectName(proj.Name)

					buildCommands = append(buildCommands, command)
				}
			} else {
				command := xbuild.New(builder.solution.Pth).SetTarget("Build")
				command.SetConfiguration(configuration)
				command.SetPlatform(platform)

				if isArchitectureArchiveable(projectConfig.MtouchArchs) {
					command.SetBuildIpa()
					command.SetArchiveOnBuild()
				}

				buildCommands = append(buildCommands, command)
			}
		case constants.ProjectTypeMac:
			if builder.forceMDTool {
				command := mdtool.New(builder.solution.Pth).SetTarget("build")
				command.SetConfiguration(projectConfig.Configuration)
				command.SetPlatform(projectConfig.Platform)
				command.SetProjectName(proj.Name)

				buildCommands = append(buildCommands, command)

				command = mdtool.New(builder.solution.Pth).SetTarget("archive")
				command.SetConfiguration(projectConfig.Configuration)
				command.SetPlatform(projectConfig.Platform)
				command.SetProjectName(proj.Name)

				buildCommands = append(buildCommands, command)
			} else {
				command := xbuild.New(builder.solution.Pth).SetTarget("Build")
				command.SetConfiguration(configuration)
				command.SetPlatform(platform)
				command.SetArchiveOnBuild()

				buildCommands = append(buildCommands, command)
			}
		case constants.ProjectTypeAndroid:
			command := xbuild.New(proj.Pth)
			if projectConfig.SignAndroid {
				command.SetTarget("SignAndroidPackage")
			} else {
				command.SetTarget("PackageForAndroid")
			}

			command.SetConfiguration(projectConfig.Configuration)

			if !isPlatformAnyCPU(projectConfig.Platform) {
				command.SetPlatform(projectConfig.Platform)
			}

			buildCommands = append(buildCommands, command)
		}

		// Run build command
		for _, buildCommand := range buildCommands {
			if callback != nil {
				callback(proj, buildCommand)
			}

			if err := buildCommand.Run(); err != nil {
				return warnings, err
			}
		}
	}

	return warnings, nil
}

// CollectOutput ...
func (builder Model) CollectOutput(configuration, platform string) (OutputMap, []string) {
	outputMap := OutputMap{}
	warnings := []string{}

	buildableProjects := builder.buildableProjects(configuration, platform)

	solutionConfig := utility.ToConfig(configuration, platform)

	for _, proj := range buildableProjects {
		projectConfigKey, ok := proj.ConfigMap[solutionConfig]
		if !ok {
			// fmt.Sprintf("project (%s) do not have config for solution config (%s), skipping...", proj.Name, solutionConfig)
			continue
		}

		projectConfig, ok := proj.Configs[projectConfigKey]
		if !ok {
			// fmt.Sprintf("project (%s) contains mapping for solution config (%s), but does not have project configuration", proj.Name, solutionConfig)
			continue
		}

		projectTypeOutputMap, ok := outputMap[proj.ProjectType]
		if !ok {
			projectTypeOutputMap = map[constants.OutputType]string{}
		}

		switch proj.ProjectType {
		case constants.ProjectTypeIos, constants.ProjectTypeTVOs:
			if xcarchivePth, err := exportLatestXCArchiveFromXcodeArchives(proj.AssemblyName); err != nil {
				warnings = append(warnings, err.Error())
			} else if xcarchivePth != "" {
				projectTypeOutputMap[constants.OutputTypeXCArchive] = xcarchivePth
			}
			if ipaPth, err := exportIpa(projectConfig.OutputDir, proj.AssemblyName); err != nil {
				warnings = append(warnings, err.Error())
			} else if ipaPth != "" {
				projectTypeOutputMap[constants.OutputTypeIPA] = ipaPth
			}
			if dsymPth, err := exportDSYM(projectConfig.OutputDir, proj.AssemblyName); err != nil {
				warnings = append(warnings, err.Error())
			} else if dsymPth != "" {
				projectTypeOutputMap[constants.OutputTypeDSYM] = dsymPth
			}
		case constants.ProjectTypeMac:
			if xcarchivePth, err := exportLatestXCArchiveFromXcodeArchives(proj.AssemblyName); err != nil {
				warnings = append(warnings, err.Error())
			} else if xcarchivePth != "" {
				projectTypeOutputMap[constants.OutputTypeXCArchive] = xcarchivePth
			}
			if appPth, err := exportApp(projectConfig.OutputDir, proj.AssemblyName); err != nil {
				warnings = append(warnings, err.Error())
			} else if appPth != "" {
				projectTypeOutputMap[constants.OutputTypeAPP] = appPth
			}
			if pkgPth, err := exportPkg(projectConfig.OutputDir, proj.AssemblyName); err != nil {
				warnings = append(warnings, err.Error())
			} else if pkgPth != "" {
				projectTypeOutputMap[constants.OutputTypePKG] = pkgPth
			}
		case constants.ProjectTypeAndroid:
			if apkPth, err := exportApk(projectConfig.OutputDir, proj.ManifestPth, projectConfig.SignAndroid); err != nil {
				warnings = append(warnings, err.Error())
			} else if apkPth != "" {
				projectTypeOutputMap[constants.OutputTypeAPK] = apkPth
			}
		}

		if len(projectTypeOutputMap) > 0 {
			outputMap[proj.ProjectType] = projectTypeOutputMap
		}
	}

	return outputMap, warnings
}
