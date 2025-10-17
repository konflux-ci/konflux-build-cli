package common

import (
	"fmt"
	"os"
	"reflect"

	l "github.com/konflux-ci/konflux-build-cli/pkg/logger"
)

// ReadResultFilesPath fills the given results path struct with file path defined in env vars.
// Each field of the result path struct must be of string type and have 'env' tag.
func ReadResultFilesPath(resultFilesPath interface{}) error {
	resultsStruct := reflect.ValueOf(resultFilesPath).Elem()
	paramsStructType := resultsStruct.Type()

	for i := 0; i < resultsStruct.NumField(); i++ {
		field := paramsStructType.Field(i)
		fieldVal := resultsStruct.Field(i)

		if fieldVal.Kind() != reflect.String {
			panic(fmt.Sprintf("ReadResultFilesPath: result '%s' path is not of string type", field.Name))
		}

		envVarName := field.Tag.Get("env")
		if envVarName == "" {
			panic(fmt.Sprintf("ReadResultFilesPath: result '%s' path 'env' tag is not set", field.Name))
		}

		envValue := os.Getenv(envVarName)
		if envValue == "" {
			return fmt.Errorf("ReadResultFilesPath: environment variable '%s' for '%s' result is not set", envVarName, field.Name)
		}

		if fieldVal.CanSet() {
			fieldVal.SetString(envValue)
		} else {
			panic(fmt.Sprintf("ReadResultFilesPath: cannot set value for '%s' field", field.Name))
		}
	}

	return nil
}

type ResultsWriterInterface interface {
	WriteResultString(result, path string) error
}

var _ ResultsWriterInterface = &ResultsWriter{}

type ResultsWriter struct {
	Verbose bool
}

func NewResultsWriter(verbose bool) *ResultsWriter {
	return &ResultsWriter{Verbose: verbose}
}

// WriteResultString writes result data into file by given path
func (r *ResultsWriter) WriteResultString(result, path string) error {
	if err := os.WriteFile(path, []byte(result), 0644); err != nil {
		return fmt.Errorf("failed to write into result file '%s': %w", path, err)
	}

	if r.Verbose {
		l.Logger.Infof("Wrote result into '%s': \n%s", path, result)
	}

	return nil
}
