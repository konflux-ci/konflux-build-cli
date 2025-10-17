package common

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestReadResultFilesPath(t *testing.T) {

	t.Run("should populate struct fields from environment variables", func(t *testing.T) {
		g := NewWithT(t)

		type TestStruct struct {
			OutputPath string `env:"OUTPUT_PATH"`
			LogPath    string `env:"LOG_PATH"`
			ResultPath string `env:"RESULT_PATH"`
		}

		os.Setenv("OUTPUT_PATH", "/tmp/output")
		os.Setenv("LOG_PATH", "/tmp/log")
		os.Setenv("RESULT_PATH", "/tmp/result")
		defer func() {
			os.Unsetenv("OUTPUT_PATH")
			os.Unsetenv("LOG_PATH")
			os.Unsetenv("RESULT_PATH")
		}()

		testStruct := &TestStruct{}
		err := ReadResultFilesPath(testStruct)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(testStruct.OutputPath).To(Equal("/tmp/output"))
		g.Expect(testStruct.LogPath).To(Equal("/tmp/log"))
		g.Expect(testStruct.ResultPath).To(Equal("/tmp/result"))
	})

	t.Run("should return error when environment variable is not set", func(t *testing.T) {
		g := NewWithT(t)

		type TestStruct struct {
			OutputPath string `env:"MISSING_ENV_VAR"`
		}

		testStruct := &TestStruct{}
		err := ReadResultFilesPath(testStruct)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("environment variable 'MISSING_ENV_VAR' for 'OutputPath' result is not set"))
	})

	t.Run("should panic when field is not string type", func(t *testing.T) {
		g := NewWithT(t)

		type TestStruct struct {
			OutputPath int `env:"OUTPUT_PATH"`
		}

		testStruct := &TestStruct{}

		g.Expect(func() {
			ReadResultFilesPath(testStruct)
		}).To(Panic())
	})

	t.Run("should panic when env tag is not set", func(t *testing.T) {
		g := NewWithT(t)

		type TestStruct struct {
			OutputPath string
		}

		testStruct := &TestStruct{}

		g.Expect(func() {
			ReadResultFilesPath(testStruct)
		}).To(Panic())
	})
}

func TestNewResultsWriter(t *testing.T) {

	t.Run("should create ResultsWriter with verbose true", func(t *testing.T) {
		g := NewWithT(t)

		writer := NewResultsWriter(true)

		g.Expect(writer).ToNot(BeNil())
		g.Expect(writer.Verbose).To(BeTrue())
	})

	t.Run("should create ResultsWriter with verbose false", func(t *testing.T) {
		g := NewWithT(t)

		writer := NewResultsWriter(false)

		g.Expect(writer).ToNot(BeNil())
		g.Expect(writer.Verbose).To(BeFalse())
	})
}

func TestResultsWriter_WriteResultString(t *testing.T) {

	t.Run("should write result to file successfully", func(t *testing.T) {
		g := NewWithT(t)

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test_result.txt")
		testContent := "test result content"

		writer := NewResultsWriter(false)
		err := writer.WriteResultString(testContent, filePath)

		g.Expect(err).ToNot(HaveOccurred())

		// Verify file was written
		content, err := os.ReadFile(filePath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(content)).To(Equal(testContent))
	})

	t.Run("should return error when file cannot be written", func(t *testing.T) {
		g := NewWithT(t)

		invalidPath := "/invalid/path/that/does/not/exist/result.txt"
		testContent := "test content"

		writer := NewResultsWriter(false)
		err := writer.WriteResultString(testContent, invalidPath)

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to write into result file"))
	})

	t.Run("should write file with correct permissions", func(t *testing.T) {
		g := NewWithT(t)

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "test_permissions.txt")
		testContent := "test content"

		writer := NewResultsWriter(false)
		err := writer.WriteResultString(testContent, filePath)

		g.Expect(err).ToNot(HaveOccurred())

		// Check file permissions
		fileInfo, err := os.Stat(filePath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(fileInfo.Mode().Perm()).To(Equal(os.FileMode(0644)))
	})
}
