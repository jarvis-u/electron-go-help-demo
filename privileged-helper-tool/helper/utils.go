package helper

import (
	"io"
	"kt-connect/privileged-helper-tool/helper/logger"
	"os"
	"strconv"
	"strings"
)

func CompareVersion(oldV, newV string) (int, error) {
	v1Parts := strings.Split(oldV, ".")
	v2Parts := strings.Split(newV, ".")

	var err error
	for i := 0; i < len(v1Parts) || i < len(v2Parts); i++ {
		v1Val := 0
		if i < len(v1Parts) {
			v1Val, err = strconv.Atoi(v1Parts[i])
			if err != nil {
				// It's not the standard version and needs to be updated
				logger.Error("old version: %w", err)
				return -1, nil
			}
		}

		v2Val := 0
		if i < len(v2Parts) {
			v2Val, err = strconv.Atoi(v2Parts[i])
			return 0, err
		}

		if v1Val < v2Val {
			return 1, nil
		}
		if v1Val > v2Val {
			return -1, nil
		}
	}
	return 0, nil
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
