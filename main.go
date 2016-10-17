package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/cmdex"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-xamarin/builder"
	"github.com/bitrise-tools/go-xamarin/constants"
	"github.com/bitrise-tools/go-xamarin/tools"
	shellquote "github.com/kballard/go-shellquote"
)

// ConfigsModel ...
type ConfigsModel struct {
	XamarinSolution      string
	XamarinConfiguration string
	XamarinPlatform      string
	ProjectTypeWhitelist string

	AndroidCustomOptions string
	IOSCustomOptions     string
	TvOSCustomOptions    string
	MacOSCustomOptions   string
	ForceMDTool          string

	DeployDir string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		XamarinSolution:      os.Getenv("xamarin_solution"),
		XamarinConfiguration: os.Getenv("xamarin_configuration"),
		XamarinPlatform:      os.Getenv("xamarin_platform"),
		ProjectTypeWhitelist: os.Getenv("project_type_whitelist"),

		AndroidCustomOptions: os.Getenv("android_build_command_custom_options"),
		IOSCustomOptions:     os.Getenv("ios_build_command_custom_options"),
		TvOSCustomOptions:    os.Getenv("tvos_build_command_custom_options"),
		MacOSCustomOptions:   os.Getenv("macos_build_command_custom_options"),
		ForceMDTool:          os.Getenv("force_mdtool"),

		DeployDir: os.Getenv("BITRISE_DEPLOY_DIR"),
	}
}

func (configs ConfigsModel) print() {
	log.Info("Configs:")

	log.Detail("- XamarinSolution: %s", configs.XamarinSolution)
	log.Detail("- XamarinConfiguration: %s", configs.XamarinConfiguration)
	log.Detail("- XamarinPlatform: %s", configs.XamarinPlatform)
	log.Detail("- ProjectTypeWhitelist: %s", configs.ProjectTypeWhitelist)

	log.Info("Experimental Configs:")

	log.Detail("- AndroidCustomOptions: %s", configs.AndroidCustomOptions)
	log.Detail("- IOSCustomOptions: %s", configs.IOSCustomOptions)
	log.Detail("- TvOSCustomOptions: %s", configs.TvOSCustomOptions)
	log.Detail("- MacOSCustomOptions: %s", configs.MacOSCustomOptions)
	log.Detail("- ForceMDTool: %s", configs.ForceMDTool)

	log.Info("Other Configs:")

	log.Detail("- DeployDir: %s", configs.DeployDir)
}

func (configs ConfigsModel) validate() error {
	if configs.XamarinSolution == "" {
		return errors.New("No XamarinSolution parameter specified!")
	}
	if exist, err := pathutil.IsPathExists(configs.XamarinSolution); err != nil {
		return fmt.Errorf("Failed to check if XamarinSolution exist at: %s, error: %s", configs.XamarinSolution, err)
	} else if !exist {
		return fmt.Errorf("XamarinSolution not exist at: %s", configs.XamarinSolution)
	}

	if configs.XamarinConfiguration == "" {
		return errors.New("No XamarinConfiguration parameter specified!")
	}

	if configs.XamarinPlatform == "" {
		return errors.New("No XamarinPlatform parameter specified!")
	}

	return nil
}

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	cmd := cmdex.NewCommand("envman", "add", "--key", keyStr)
	cmd.SetStdin(strings.NewReader(valueStr))
	return cmd.Run()
}

func exportZipedArtifactDir(pth, deployDir, envKey string) (string, error) {
	parentDir := filepath.Dir(pth)
	dirName := filepath.Base(pth)
	deployPth := filepath.Join(deployDir, dirName+".zip")
	cmd := cmdex.NewCommand("/usr/bin/zip", "-rTy", deployPth, dirName)
	cmd.SetDir(parentDir)
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed to zip dir: %s, output: %s, error: %s", pth, out, err)
	}

	if err := exportEnvironmentWithEnvman(envKey, deployPth); err != nil {
		return "", fmt.Errorf("Failed to export artifact path (%s) into (%s)", deployPth, envKey)
	}

	return deployPth, nil
}

func exportArtifactDir(pth, deployDir, envKey string) (string, error) {
	base := filepath.Base(pth)
	deployPth := filepath.Join(deployDir, base)

	if err := cmdex.CopyDir(pth, deployDir, false); err != nil {
		return "", fmt.Errorf("Failed to move artifact (%s) to (%s)", pth, deployDir)
	}

	if err := exportEnvironmentWithEnvman(envKey, deployPth); err != nil {
		return "", fmt.Errorf("Failed to export artifact path (%s) into (%s)", deployPth, envKey)
	}

	return deployPth, nil
}

func exportArtifactFile(pth, deployDir, envKey string) (string, error) {
	base := filepath.Base(pth)
	deployPth := filepath.Join(deployDir, base)

	if err := cmdex.CopyFile(pth, deployPth); err != nil {
		return "", fmt.Errorf("Failed to move artifact (%s) to (%s)", pth, deployPth)
	}

	if err := exportEnvironmentWithEnvman(envKey, deployPth); err != nil {
		return "", fmt.Errorf("Failed to export artifact path (%s) into (%s)", deployPth, envKey)
	}

	return deployPth, nil
}

func main() {
	configs := createConfigsModelFromEnvs()

	fmt.Println()
	configs.print()

	if err := configs.validate(); err != nil {
		fmt.Println()
		log.Error("Issue with input: %s", err)
		fmt.Println()

		os.Exit(1)
	}

	// parse project type filters
	projectTypeWhitelist := []constants.ProjectType{}
	if len(configs.ProjectTypeWhitelist) > 0 {
		split := strings.Split(configs.ProjectTypeWhitelist, ",")
		for _, item := range split {
			item := strings.TrimSpace(item)
			projectType, err := constants.ParseProjectType(item)
			if err != nil {
				log.Error("Failed to parse project type (%s), error: %s", item, err)
				os.Exit(1)
			}
			projectTypeWhitelist = append(projectTypeWhitelist, projectType)
		}
	}
	// ---

	// prepare custom options
	projectTypeCustomOptions := map[constants.ProjectType][]string{}
	projectTypeRawCustomOptions := map[constants.ProjectType]string{
		constants.ProjectTypeAndroid: configs.AndroidCustomOptions,
		constants.ProjectTypeIOS:     configs.IOSCustomOptions,
		constants.ProjectTypeTvOS:    configs.TvOSCustomOptions,
		constants.ProjectTypeMacOS:   configs.MacOSCustomOptions,
	}
	for projectType, rawOptions := range projectTypeRawCustomOptions {
		if rawOptions == "" {
			continue
		}

		split, err := shellquote.Split(rawOptions)
		if err != nil {
			log.Error("Failed to split options (%s), error: %s", err)
		}
		projectTypeCustomOptions[projectType] = split
	}
	// ---

	//
	// build
	fmt.Println()
	log.Info("Building all projects in solution: %s", configs.XamarinSolution)

	builder, err := builder.New(configs.XamarinSolution, projectTypeWhitelist, (configs.ForceMDTool == "yes"))
	if err != nil {
		log.Error("Failed to create xamarin builder, error: %s", err)
		os.Exit(1)
	}

	prepareCallback := func(solutionName string, projectName string, projectType constants.ProjectType, command *tools.Editable) {
		options, ok := projectTypeCustomOptions[projectType]
		if ok {
			(*command).SetCustomOptions(options...)
		}
	}

	callback := func(solutionName string, projectName string, projectType constants.ProjectType, commandStr string, alreadyPerformed bool) {
		fmt.Println()
		log.Info("Building project: %s", projectName)
		log.Done("$ %s", commandStr)
		if alreadyPerformed {
			log.Warn("build command already performed, skipping...")
		}
		fmt.Println()
	}

	warnings, err := builder.BuildAllProjects(configs.XamarinConfiguration, configs.XamarinPlatform, prepareCallback, callback)
	if len(warnings) > 0 {
		log.Warn("Build warnings:")
		for _, warning := range warnings {
			log.Warn(warning)
		}
	}
	if err != nil {
		log.Error("Build failed, error: %s", err)
		os.Exit(1)
	}

	output, err := builder.CollectProjectOutputs(configs.XamarinConfiguration, configs.XamarinPlatform)
	if err != nil {
		log.Error("Failed to collect output, error: %s", err)
		os.Exit(1)
	}
	// ---

	// Export outputs
	fmt.Println()
	log.Info("Exporting generated outputs...")

	for projectName, projectOutput := range output {
		fmt.Println()
		log.Info("%s outputs:", projectName)

		for _, output := range projectOutput.Outputs {
			// Android outputs
			if projectOutput.ProjectType == constants.ProjectTypeAndroid && output.OutputType == constants.OutputTypeAPK {
				envKey := "BITRISE_APK_PATH"
				pth, err := exportArtifactFile(output.Pth, configs.DeployDir, envKey)
				if err != nil {
					log.Error("Failed to export apk, error: %s", err)
					os.Exit(1)
				}
				log.Done("apk path (%s) is available in (%s) environment variable", pth, envKey)
			}

			// IOS outputs
			if projectOutput.ProjectType == constants.ProjectTypeIOS {
				if output.OutputType == constants.OutputTypeXCArchive {
					envKey := "BITRISE_XCARCHIVE_PATH"
					pth, err := exportArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						log.Error("Failed to export xcarchive, error: %s", err)
						os.Exit(1)
					}
					log.Done("xcarchive path (%s) is available in (%s) environment variable", pth, envKey)
				}

				if output.OutputType == constants.OutputTypeIPA {
					envKey := "BITRISE_IPA_PATH"
					pth, err := exportArtifactFile(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						log.Error("Failed to export ipa, error: %s", err)
						os.Exit(1)
					}
					log.Done("ipa path (%s) is available in (%s) environment variable", pth, envKey)
				}

				if output.OutputType == constants.OutputTypeDSYM {
					envKey := "BITRISE_DSYM_PATH"
					pth, err := exportZipedArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						log.Error("Failed to export dsym, error: %s", err)
						os.Exit(1)
					}
					log.Done("dsym path (%s) is available in (%s) environment variable", pth, envKey)
				}
			}

			// TvOS outputs
			if projectOutput.ProjectType == constants.ProjectTypeTvOS {
				if output.OutputType == constants.OutputTypeXCArchive {
					envKey := "BITRISE_TVOS_XCARCHIVE_PATH"
					pth, err := exportArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						log.Error("Failed to export xcarchive, error: %s", err)
						os.Exit(1)
					}
					log.Done("xcarchive path (%s) is available in (%s) environment variable", pth, envKey)
				}

				if output.OutputType == constants.OutputTypeIPA {
					envKey := "BITRISE_TVOS_IPA_PATH"
					pth, err := exportArtifactFile(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						log.Error("Failed to export ipa, error: %s", err)
						os.Exit(1)
					}
					log.Done("ipa path (%s) is available in (%s) environment variable", pth, envKey)
				}

				if output.OutputType == constants.OutputTypeDSYM {
					envKey := "BITRISE_TVOS_DSYM_PATH"
					pth, err := exportZipedArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						log.Error("Failed to export dsym, error: %s", err)
						os.Exit(1)
					}
					log.Done("dsym path (%s) is available in (%s) environment variable", pth, envKey)
				}
			}

			// MacOS outputs
			if projectOutput.ProjectType == constants.ProjectTypeMacOS {
				if output.OutputType == constants.OutputTypeXCArchive {
					envKey := "BITRISE_MACOS_XCARCHIVE_PATH"
					pth, err := exportArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						log.Error("Failed to export xcarchive, error: %s", err)
						os.Exit(1)
					}
					log.Done("xcarchive path (%s) is available in (%s) environment variable", pth, envKey)
				}

				if output.OutputType == constants.OutputTypeAPP {
					envKey := "BITRISE_MACOS_APP_PATH"
					pth, err := exportArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						log.Error("Failed to export xcarchive, error: %s", err)
						os.Exit(1)
					}
					log.Done("app path (%s) is available in (%s) environment variable", pth, envKey)
				}

				if output.OutputType == constants.OutputTypePKG {
					envKey := "BITRISE_MACOS_PKG_PATH"
					pth, err := exportArtifactFile(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						log.Error("Failed to export pkg, error: %s", err)
						os.Exit(1)
					}
					log.Done("pkg path (%s) is available in (%s) environment variable", pth, envKey)
				}
			}
		}
	}
	// ---
}
