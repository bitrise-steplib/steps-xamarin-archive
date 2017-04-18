package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/command"
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
	log.Infof("Configs:")

	log.Printf("- XamarinSolution: %s", configs.XamarinSolution)
	log.Printf("- XamarinConfiguration: %s", configs.XamarinConfiguration)
	log.Printf("- XamarinPlatform: %s", configs.XamarinPlatform)
	log.Printf("- ProjectTypeWhitelist: %s", configs.ProjectTypeWhitelist)

	log.Infof("Experimental Configs:")

	log.Printf("- AndroidCustomOptions: %s", configs.AndroidCustomOptions)
	log.Printf("- IOSCustomOptions: %s", configs.IOSCustomOptions)
	log.Printf("- TvOSCustomOptions: %s", configs.TvOSCustomOptions)
	log.Printf("- MacOSCustomOptions: %s", configs.MacOSCustomOptions)
	log.Printf("- ForceMDTool: %s", configs.ForceMDTool)

	log.Infof("Other Configs:")

	log.Printf("- DeployDir: %s", configs.DeployDir)
}

func (configs ConfigsModel) validate() error {
	if configs.XamarinSolution == "" {
		return errors.New("no XamarinSolution parameter specified")
	}
	if exist, err := pathutil.IsPathExists(configs.XamarinSolution); err != nil {
		return fmt.Errorf("failed to check if XamarinSolution exist at: %s, error: %s", configs.XamarinSolution, err)
	} else if !exist {
		return fmt.Errorf("XamarinSolution not exist at: %s", configs.XamarinSolution)
	}

	if configs.XamarinConfiguration == "" {
		return errors.New("no XamarinConfiguration parameter specified")
	}

	if configs.XamarinPlatform == "" {
		return errors.New("no XamarinPlatform parameter specified")
	}

	return nil
}

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	cmd := command.New("envman", "add", "--key", keyStr)
	cmd.SetStdin(strings.NewReader(valueStr))
	return cmd.Run()
}

func exportZipedArtifactDir(pth, deployDir, envKey string) (string, error) {
	parentDir := filepath.Dir(pth)
	dirName := filepath.Base(pth)
	deployPth := filepath.Join(deployDir, dirName+".zip")
	cmd := command.New("/usr/bin/zip", "-rTy", deployPth, dirName)
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

	if err := command.CopyDir(pth, deployDir, false); err != nil {
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

	if err := command.CopyFile(pth, deployPth); err != nil {
		return "", fmt.Errorf("Failed to move artifact (%s) to (%s)", pth, deployPth)
	}

	if err := exportEnvironmentWithEnvman(envKey, deployPth); err != nil {
		return "", fmt.Errorf("Failed to export artifact path (%s) into (%s)", deployPth, envKey)
	}

	return deployPth, nil
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func main() {
	configs := createConfigsModelFromEnvs()

	fmt.Println()
	configs.print()

	if err := configs.validate(); err != nil {
		fmt.Println()
		failf("Issue with input: %s", err)
	}

	// parse project type filters
	projectTypeWhitelist := []constants.SDK{}
	if len(configs.ProjectTypeWhitelist) > 0 {
		split := strings.Split(configs.ProjectTypeWhitelist, ",")

		for _, item := range split {
			item := strings.TrimSpace(item)
			if item == "" {
				continue
			}

			projectType, err := constants.ParseSDK(item)
			if err != nil {
				failf("Failed to parse project type (%s), error: %s", item, err)
			}

			projectTypeWhitelist = append(projectTypeWhitelist, projectType)
		}
	}
	// ---

	// prepare custom options
	projectTypeCustomOptions := map[constants.SDK][]string{}
	projectTypeRawCustomOptions := map[constants.SDK]string{
		constants.SDKAndroid: configs.AndroidCustomOptions,
		constants.SDKIOS:     configs.IOSCustomOptions,
		constants.SDKTvOS:    configs.TvOSCustomOptions,
		constants.SDKMacOS:   configs.MacOSCustomOptions,
	}
	for projectType, rawOptions := range projectTypeRawCustomOptions {
		if rawOptions == "" {
			continue
		}

		split, err := shellquote.Split(rawOptions)
		if err != nil {
			log.Errorf("failed to split options (%s), error: %s", rawOptions, err)
		}
		projectTypeCustomOptions[projectType] = split
	}
	// ---

	//
	// build
	fmt.Println()
	log.Infof("Building all projects in solution: %s", configs.XamarinSolution)

	builder, err := builder.New(configs.XamarinSolution, projectTypeWhitelist, (configs.ForceMDTool == "yes"))
	if err != nil {
		failf("Failed to create xamarin builder, error: %s", err)
	}

	prepareCallback := func(solutionName string, projectName string, sdk constants.SDK, testFramwork constants.TestFramework, command *tools.Editable) {
		options, ok := projectTypeCustomOptions[sdk]
		if ok {
			(*command).SetCustomOptions(options...)
		}
	}

	callback := func(solutionName string, projectName string, sdk constants.SDK, testFramwork constants.TestFramework, commandStr string, alreadyPerformed bool) {
		fmt.Println()
		log.Infof("Building project: %s", projectName)
		log.Donef("$ %s", commandStr)
		if alreadyPerformed {
			log.Warnf("build command already performed, skipping...")
		}
		fmt.Println()
	}

	startTime := time.Now()

	warnings, err := builder.BuildAllProjects(configs.XamarinConfiguration, configs.XamarinPlatform, prepareCallback, callback)
	if len(warnings) > 0 {
		log.Warnf("Build warnings:")
		for _, warning := range warnings {
			log.Warnf(warning)
		}
	}
	if err != nil {
		failf("Build failed, error: %s", err)
	}

	endTime := time.Now()

	output, err := builder.CollectProjectOutputs(configs.XamarinConfiguration, configs.XamarinPlatform, startTime, endTime)
	if err != nil {
		failf("Failed to collect output, error: %s", err)
	}

	if len(output) == 0 {
		failf("No output generated")
	}
	// ---

	// Export outputs
	fmt.Println()
	log.Infof("Exporting generated outputs...")

	for projectName, projectOutput := range output {
		fmt.Println()
		log.Donef("%s outputs:", projectName)

		for _, output := range projectOutput.Outputs {
			// Android outputs
			if projectOutput.ProjectType == constants.SDKAndroid && output.OutputType == constants.OutputTypeAPK {
				envKey := "BITRISE_APK_PATH"
				pth, err := exportArtifactFile(output.Pth, configs.DeployDir, envKey)
				if err != nil {
					failf("Failed to export apk, error: %s", err)
				}
				fmt.Println()
				log.Printf("The apk path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
			}

			// IOS outputs
			if projectOutput.ProjectType == constants.SDKIOS {
				if output.OutputType == constants.OutputTypeXCArchive {
					envKey := "BITRISE_XCARCHIVE_PATH"
					pth, err := exportArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						failf("Failed to export xcarchive, error: %s", err)
					}
					fmt.Println()
					log.Printf("The xcarchive path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
				}

				if output.OutputType == constants.OutputTypeIPA {
					envKey := "BITRISE_IPA_PATH"
					pth, err := exportArtifactFile(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						failf("Failed to export ipa, error: %s", err)
					}
					fmt.Println()
					log.Printf("The ipa path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
				}

				if output.OutputType == constants.OutputTypeDSYM {
					envKey := "BITRISE_DSYM_PATH"
					pth, err := exportZipedArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						failf("Failed to export dsym, error: %s", err)
					}
					fmt.Println()
					log.Printf("The dsym zip path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
				}

				if output.OutputType == constants.OutputTypeAPP {
					envKey := "BITRISE_APP_PATH"
					pth, err := exportArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						failf("Failed to export app, error: %s", err)
					}
					fmt.Println()
					log.Printf("The app path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
				}
			}

			// TvOS outputs
			if projectOutput.ProjectType == constants.SDKTvOS {
				if output.OutputType == constants.OutputTypeXCArchive {
					envKey := "BITRISE_TVOS_XCARCHIVE_PATH"
					pth, err := exportArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						failf("Failed to export xcarchive, error: %s", err)
					}
					fmt.Println()
					log.Printf("The xcarchive path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
				}

				if output.OutputType == constants.OutputTypeIPA {
					envKey := "BITRISE_TVOS_IPA_PATH"
					pth, err := exportArtifactFile(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						failf("Failed to export ipa, error: %s", err)
					}
					fmt.Println()
					log.Printf("The ipa path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
				}

				if output.OutputType == constants.OutputTypeDSYM {
					envKey := "BITRISE_TVOS_DSYM_PATH"
					pth, err := exportZipedArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						failf("Failed to export dsym, error: %s", err)
					}
					fmt.Println()
					log.Printf("The dsym zip path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
				}

				if output.OutputType == constants.OutputTypeAPP {
					envKey := "BITRISE_TVOS_APP_PATH"
					pth, err := exportArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						failf("Failed to export app, error: %s", err)
					}
					fmt.Println()
					log.Printf("The app path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
				}
			}

			// MacOS outputs
			if projectOutput.ProjectType == constants.SDKMacOS {
				if output.OutputType == constants.OutputTypeXCArchive {
					envKey := "BITRISE_MACOS_XCARCHIVE_PATH"
					pth, err := exportArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						failf("Failed to export xcarchive, error: %s", err)
					}
					fmt.Println()
					log.Printf("The xcarchive path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
				}

				if output.OutputType == constants.OutputTypeAPP {
					envKey := "BITRISE_MACOS_APP_PATH"
					pth, err := exportArtifactDir(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						failf("Failed to export app, error: %s", err)
					}
					fmt.Println()
					log.Printf("The app path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
				}

				if output.OutputType == constants.OutputTypePKG {
					envKey := "BITRISE_MACOS_PKG_PATH"
					pth, err := exportArtifactFile(output.Pth, configs.DeployDir, envKey)
					if err != nil {
						failf("Failed to export pkg, error: %s", err)
					}
					fmt.Println()
					log.Printf("The pkg path is now available in the Environment Variable: %s\nvalue: %s", envKey, pth)
				}
			}
		}
	}
	// ---
}
