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
	"github.com/bitrise-tools/go-xamarin/buildtool"
	"github.com/bitrise-tools/go-xamarin/constants"
	"github.com/bitrise-tools/go-xamarin/project"
)

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	cmd := cmdex.NewCommand("envman", "add", "--key", keyStr)
	cmd.SetStdin(strings.NewReader(valueStr))
	return cmd.Run()
}

func exportZipedArtifactDir(pth, deployDir, envKey string) error {
	parentDir := filepath.Dir(pth)
	dirName := filepath.Base(pth)
	deployPth := filepath.Join(deployDir, dirName+".zip")
	cmd := cmdex.NewCommand("/usr/bin/zip", "-rTy", deployPth, dirName)
	cmd.SetDir(parentDir)
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to zip dir: %s, output: %s, error: %s", pth, out, err)
	}

	if err := exportEnvironmentWithEnvman(envKey, deployPth); err != nil {
		return fmt.Errorf("Failed to export artifact path (%s) into (%s)", deployPth, envKey)
	}

	log.Done("artifact path (%s) is available in (%s) environment variable", deployPth, envKey)

	return nil
}

func exportArtifactDir(pth, deployDir, envKey string) error {
	base := filepath.Base(pth)
	deployPth := filepath.Join(deployDir, base)

	if err := cmdex.CopyDir(pth, deployPth, false); err != nil {
		return fmt.Errorf("Failed to move artifact (%s) to (%s)", pth, deployPth)
	}

	if err := exportEnvironmentWithEnvman(envKey, deployPth); err != nil {
		return fmt.Errorf("Failed to export artifact path (%s) into (%s)", deployPth, envKey)
	}

	log.Done("artifact path (%s) is available in (%s) environment variable", deployPth, envKey)

	return nil
}

func exportArtifactFile(pth, deployDir, envKey string) error {
	base := filepath.Base(pth)
	deployPth := filepath.Join(deployDir, base)

	if err := cmdex.CopyFile(pth, deployPth); err != nil {
		return fmt.Errorf("Failed to move artifact (%s) to (%s)", pth, deployPth)
	}

	if err := exportEnvironmentWithEnvman(envKey, deployPth); err != nil {
		return fmt.Errorf("Failed to export artifact path (%s) into (%s)", deployPth, envKey)
	}

	log.Done("artifact path (%s) is available in (%s) environment variable", deployPth, envKey)

	return nil
}

// ConfigsModel ...
type ConfigsModel struct {
	XamarinSolution      string
	XamarinConfiguration string
	XamarinPlatform      string
	ProjectTypeWhitelist string
	ForceMDTool          string

	// Other configs
	DeployDir string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		XamarinSolution:      os.Getenv("xamarin_solution"),
		XamarinConfiguration: os.Getenv("xamarin_configuration"),
		XamarinPlatform:      os.Getenv("xamarin_platform"),
		ProjectTypeWhitelist: os.Getenv("project_type_whitelist"),
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
	log.Detail("- ForceMDTool: %s", configs.ForceMDTool)
	fmt.Println()

	log.Detail("- DeployDir: %s", configs.DeployDir)
}

func (configs ConfigsModel) validate() error {
	// required
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

	// build
	fmt.Println()
	log.Info("Building all projects in solution: %s", configs.XamarinSolution)

	builder, err := builder.New(configs.XamarinSolution, projectTypeWhitelist, (configs.ForceMDTool == "yes"))
	if err != nil {
		log.Error("Failed to create xamarin builder, error: %s", err)
		os.Exit(1)
	}

	callback := func(project project.Model, command buildtool.PrintableCommand) {
		fmt.Println()
		log.Info("Building project: %s", project.Name)
		log.Done("$ %s", command.PrintableCommand())
		fmt.Println()
	}

	warnings, err := builder.BuildAllProjects(configs.XamarinConfiguration, configs.XamarinPlatform, callback)
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

	output, warnings := builder.CollectOutput(configs.XamarinConfiguration, configs.XamarinPlatform)
	if len(warnings) > 0 {
		log.Warn("Build warnings:")
		for _, warning := range warnings {
			log.Warn(warning)
		}
	}
	// ---

	// export outputs
	fmt.Println()
	log.Info("Exporting generated outputs...")

	for projectType, outputMap := range output {
		fmt.Println()
		log.Info("%s outputs:", projectType)

		switch projectType {
		case constants.ProjectTypeIos:
			xcarchivePth, ok := outputMap[constants.OutputTypeXCArchive]
			if ok {
				log.Detail("exporintg iOS xcarchive: %s", xcarchivePth)
				if err := exportArtifactDir(xcarchivePth, configs.DeployDir, "BITRISE_IOS_XCARCHIVE_PATH"); err != nil {
					log.Error("Failed to export xcarchive, error: %s", err)
					os.Exit(1)
				}
			}
			ipaPth, ok := outputMap[constants.OutputTypeIPA]
			if ok {
				log.Detail("exporintg iOS ipa: %s", ipaPth)
				if err := exportArtifactFile(ipaPth, configs.DeployDir, "BITRISE_IOS_IPA_PATH"); err != nil {
					log.Error("Failed to export ipa, error: %s", err)
					os.Exit(1)
				}
			}
			dsymPth, ok := outputMap[constants.OutputTypeDSYM]
			if ok {
				log.Detail("exporintg iOS dSYM: %s", dsymPth)
				if err := exportZipedArtifactDir(dsymPth, configs.DeployDir, "BITRISE_IOS_DSYM_PATH"); err != nil {
					log.Error("Failed to export dsym, error: %s", err)
					os.Exit(1)
				}
			}
		case constants.ProjectTypeAndroid:
			apkPth, ok := outputMap[constants.OutputTypeAPK]
			if ok {
				log.Detail("exporintg apk: %s", apkPth)
				if err := exportArtifactFile(apkPth, configs.DeployDir, "BITRISE_APK_PATH"); err != nil {
					log.Error("Failed to export apk, error: %s", err)
					os.Exit(1)
				}
			}
		case constants.ProjectTypeMac:
			xcarchivePth, ok := outputMap[constants.OutputTypeXCArchive]
			if ok {
				log.Detail("exporintg macOS xcarchive: %s", xcarchivePth)
				if err := exportArtifactDir(xcarchivePth, configs.DeployDir, "BITRISE_MACOS_XCARCHIVE_PATH"); err != nil {
					log.Error("Failed to export xcarchive, error: %s", err)
					os.Exit(1)
				}
			}
			appPth, ok := outputMap[constants.OutputTypeAPP]
			if ok {
				log.Detail("exporintg macOS app: %s", appPth)
				if err := exportArtifactDir(appPth, configs.DeployDir, "BITRISE_MACOS_APP_PATH"); err != nil {
					log.Error("Failed to export xcarchive, error: %s", err)
					os.Exit(1)
				}
			}
			pkgPth, ok := outputMap[constants.OutputTypePKG]
			if ok {
				log.Detail("exporintg macOS pkg: %s", pkgPth)
				if err := exportArtifactFile(pkgPth, configs.DeployDir, "BITRISE_MACOS_PKG_PATH"); err != nil {
					log.Error("Failed to export pkg, error: %s", err)
					os.Exit(1)
				}
			}
		case constants.ProjectTypeTVOs:
			xcarchivePth, ok := outputMap[constants.OutputTypeXCArchive]
			if ok {
				log.Detail("exporintg tvOS xcarchive: %s", xcarchivePth)
				if err := exportArtifactDir(xcarchivePth, configs.DeployDir, "BITRISE_TVOS_XCARCHIVE_PATH"); err != nil {
					log.Error("Failed to export xcarchive, error: %s", err)
					os.Exit(1)
				}
			}
			ipaPth, ok := outputMap[constants.OutputTypeIPA]
			if ok {
				log.Detail("exporintg tvOS ipa: %s", ipaPth)
				if err := exportArtifactFile(ipaPth, configs.DeployDir, "BITRISE_TVOS_IPA_PATH"); err != nil {
					log.Error("Failed to export ipa, error: %s", err)
					os.Exit(1)
				}
			}
			dsymPth, ok := outputMap[constants.OutputTypeDSYM]
			if ok {
				log.Detail("exporintg tvOS dSYM: %s", dsymPth)
				if err := exportZipedArtifactDir(dsymPth, configs.DeployDir, "BITRISE_TVOS_DSYM_PATH"); err != nil {
					log.Error("Failed to export dsym, error: %s", err)
					os.Exit(1)
				}
			}
		}
	}
	// ---
}
