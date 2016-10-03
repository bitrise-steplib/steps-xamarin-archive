package builder

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-tools/go-xamarin/constants"
	"github.com/bitrise-tools/go-xamarin/solution"
	"github.com/bitrise-tools/go-xamarin/utility"
)

const (
	packageNamePattern = `(?i)<manifest.*package="(?P<package_name>.*)">`
)

func validateSolutionPth(pth string) error {
	ext := filepath.Ext(pth)
	if ext != constants.SolutionExt {
		return fmt.Errorf("path is not a solution file path: %s", pth)
	}
	if exist, err := pathutil.IsPathExists(pth); err != nil {
		return err
	} else if !exist {
		return fmt.Errorf("solution not exist at: %s", pth)
	}
	return nil
}

func validateSolutionConfig(solution solution.Model, configuration, platform string) error {
	config := utility.ToConfig(configuration, platform)
	if _, ok := solution.ConfigMap[config]; !ok {
		return fmt.Errorf("invalid solution config, available: %v", solution.ConfigList())
	}
	return nil
}

func isProjectTypeAllowed(projectType constants.ProjectType, projectTypeWhiteList ...constants.ProjectType) bool {
	if len(projectTypeWhiteList) == 0 {
		return true
	}

	for _, filter := range projectTypeWhiteList {
		switch filter {
		case constants.ProjectTypeIos:
			if projectType == constants.ProjectTypeIos {
				return true
			}
		case constants.ProjectTypeTVOs:
			if projectType == constants.ProjectTypeTVOs {
				return true
			}
		case constants.ProjectTypeMac:
			if projectType == constants.ProjectTypeMac {
				return true
			}
		case constants.ProjectTypeAndroid:
			if projectType == constants.ProjectTypeAndroid {
				return true
			}
		}
	}

	return false
}

func isArchitectureArchiveable(architectures []string) bool {
	// default is armv7
	if len(architectures) == 0 {
		return true
	}

	for _, arch := range architectures {
		arch = strings.ToLower(arch)
		if !strings.HasPrefix(arch, "arm") {
			return false
		}
	}

	return true
}

func androidPackageName(manifestPth string) (string, error) {
	packageName := ""

	content, err := fileutil.ReadStringFromFile(manifestPth)
	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := regexp.MustCompile(packageNamePattern).FindStringSubmatch(line); len(matches) == 2 {
			packageName = matches[1]
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return packageName, nil
}

func exportApk(outputDir, manifestPth string, isSigned bool) (string, error) {
	apkPth := ""

	packageName, err := androidPackageName(manifestPth)
	if err != nil {
		return "", err
	}

	apks, err := filepath.Glob(filepath.Join(outputDir, "*.apk"))
	if err != nil {
		return "", err
	}

	if isSigned {
		pattern := fmt.Sprintf(`(?i)%s.*signed\.apk`, packageName)

		for _, apk := range apks {
			if match := regexp.MustCompile(pattern).FindString(apk); match != "" {
				apkPth = apk
			}
		}
	}

	if apkPth == "" {
		pattern := fmt.Sprintf(`(?i)%s.*\.apk`, packageName)

		for _, apk := range apks {
			if match := regexp.MustCompile(pattern).FindString(apk); match != "" {
				apkPth = apk
			}
		}
	}

	if apkPth == "" {
		pattern := fmt.Sprintf(`(?i).*\.apk`)

		for _, apk := range apks {
			if match := regexp.MustCompile(pattern).FindString(apk); match != "" {
				apkPth = apk
			}
		}
	}

	return apkPth, nil
}

func exportLatestXCArchiveFromXcodeArchives(projectName string) (string, error) {
	userHomeDir := os.Getenv("HOME")
	if userHomeDir == "" {
		return "", fmt.Errorf("failed to get user home dir")
	}
	xcodeArchivesDir := filepath.Join(userHomeDir, "Library/Developer/Xcode/Archives")
	if exist, err := pathutil.IsDirExists(xcodeArchivesDir); err != nil {
		return "", err
	} else if !exist {
		return "", fmt.Errorf("no default Xcode archive path found at: %s", xcodeArchivesDir)
	}

	latestArchive := ""
	latestArchiveDate := time.Time{}

	pattern := filepath.Join(xcodeArchivesDir, "*", projectName+" *.xcarchive")

	archives, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	for _, archive := range archives {
		base := filepath.Base(archive)
		ext := filepath.Ext(archive)
		dateStr := strings.TrimPrefix(base, projectName)
		dateStr = strings.TrimSuffix(dateStr, ext)
		dateStr = strings.TrimSpace(dateStr)

		if strings.Contains(dateStr, "AM") {
			split := strings.SplitAfter(dateStr, "AM")
			if len(split) > 0 {
				dateStr = split[0]
			}
		}

		if strings.Contains(dateStr, "PM") {
			split := strings.SplitAfter(dateStr, "PM")
			if len(split) > 0 {
				dateStr = split[0]
			}
		}

		archiveDate, err := time.Parse("1-2-06 3.04 PM", dateStr)
		if err != nil {
			return "", err
		}

		if latestArchive == "" || archiveDate.After(latestArchiveDate) {
			latestArchive = archive
			latestArchiveDate = archiveDate
		}
	}

	return latestArchive, nil
}

func exportIpa(outputDir, assemblyName string) (string, error) {
	pattern := filepath.Join(outputDir, assemblyName+"*", assemblyName+".ipa")
	ipas, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(ipas) == 0 {
		pattern := filepath.Join(outputDir, "*", "*.ipa")
		ipas, err = filepath.Glob(pattern)
		if err != nil {
			return "", err
		}

		if len(ipas) == 0 {
			return "", nil
		}
	}

	latestIpaPth := ""
	latestIpaDate := time.Time{}

	datePattern := ".* ([0-9]+-[0-9]+-[0-9]+ [0-9]+-[0-9]+-[0-9]+).*"
	regexp := regexp.MustCompile(datePattern)

	for _, ipa := range ipas {
		matches := regexp.FindStringSubmatch(ipa)
		if len(matches) > 1 {
			dateStr := matches[1]

			ipaDate, err := time.Parse("2006-01-02 15-04-05", dateStr)
			if err != nil {
				return "", err
			}

			if latestIpaPth == "" || ipaDate.After(latestIpaDate) {
				latestIpaPth = ipa
				latestIpaDate = ipaDate
			}
		}
	}

	return latestIpaPth, nil
}

func exportDSYM(outputDir, assemblyName string) (string, error) {
	pattern := filepath.Join(outputDir, assemblyName+"*.dSYM")
	dSYMs, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(dSYMs) == 0 {
		pattern := filepath.Join(outputDir, "*.dSYM")
		dSYMs, err = filepath.Glob(pattern)
		if err != nil {
			return "", err
		}

		if len(dSYMs) == 0 {
			return "", nil
		}
	}
	return dSYMs[0], nil
}

func exportPkg(outputDir, assemblyName string) (string, error) {
	pattern := filepath.Join(outputDir, assemblyName+"*.pkg")
	pkgs, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(pkgs) == 0 {
		return "", nil
	}
	return pkgs[0], nil
}

func exportApp(outputDir, assemblyName string) (string, error) {
	pattern := filepath.Join(outputDir, assemblyName+"*.app")
	apps, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(apps) == 0 {
		return "", nil
	}
	return apps[0], nil
}

func isPlatformAnyCPU(platform string) bool {
	return (platform == "Any CPU" || platform == "AnyCPU")
}
