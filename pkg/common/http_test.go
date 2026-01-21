package common

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/h2non/gock"
)

func TestFetchFile(t *testing.T) {
	defer gock.Off()

	gock.New("https://scm.io").
		Get("/namespace/app/Dockerfile").
		Reply(200).
		BodyString("FROM fedora")

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "Dockerfile")

	if err := FetchFile("https://scm.io/namespace/app/Dockerfile", testFile, 0); err != nil {
		t.Fatalf("FetchFile failed: %v", err)
	}

	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatal("Expected file was not created")
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	expected := "FROM fedora"
	if string(content) != expected {
		t.Errorf("Expected file content '%s', but got '%s'", expected, string(content))
	}

	if !gock.IsDone() {
		t.Error("Not all mocked HTTP requests were called")
	}
}

func TestFetchFileErrorOnNon200(t *testing.T) {
	defer gock.Off()

	gock.New("https://scm.io").
		Get("/namespace/app/Dockerfile").
		Reply(500).
		BodyString("Internal Server Error")

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "Dockerfile")

	err := FetchFile("https://scm.io/namespace/app/Dockerfile", testFile, 0)
	if err == nil {
		t.Fatal("Expected FetchFile to return an error for non-200 status code")
	}

	expectedError := "File fetch HTTP request failure. Server response status: 500"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', but got '%s'", expectedError, err.Error())
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File should not be created when HTTP request fails")
	}

	if !gock.IsDone() {
		t.Error("Not all mocked HTTP requests were called")
	}
}

func TestFetchFileErrorOnSizeLimitExceeded(t *testing.T) {
	defer gock.Off()

	largeContent := make([]byte, 100)
	for i := range largeContent {
		largeContent[i] = 'A'
	}

	gock.New("https://scm.io").
		Get("/namespace/app/large-file").
		Reply(200).
		Body(bytes.NewReader(largeContent))

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "large-file")

	err := FetchFile("https://scm.io/namespace/app/large-file", testFile, 50)
	if err == nil {
		t.Fatal("Expected FetchFile to return an error when actual content exceeds size limit")
	}

	expectedErrorMsg := "Remote file size exceeds maximum allowed size of 50 bytes"
	if err.Error() != expectedErrorMsg {
		t.Errorf("Expected error message '%s', but got '%s'", expectedErrorMsg, err.Error())
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File should not be created when size limit is exceeded")
	}

	if !gock.IsDone() {
		t.Error("Not all mocked HTTP requests were called")
	}
}
