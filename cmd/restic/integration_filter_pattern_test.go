package main

import (
	"os"
	"path/filepath"
	"testing"

	rtest "github.com/chanhpng/vlbe/internal/test"
)

func TestBackupFailsWhenUsingInvalidPatterns(t *testing.T) {
	env, cleanup := withTestEnvironment(t)
	defer cleanup()

	testRunInit(t, env.gopts)

	var err error

	// Test --exclude
	err = testRunBackupAssumeFailure(t, filepath.Dir(env.testdata), []string{"testdata"}, BackupOptions{excludePatternOptions: excludePatternOptions{Excludes: []string{"*[._]log[.-][0-9]", "!*[._]log[.-][0-9]"}}}, env.gopts)

	rtest.Equals(t, `Fatal: --exclude: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())

	// Test --iexclude
	err = testRunBackupAssumeFailure(t, filepath.Dir(env.testdata), []string{"testdata"}, BackupOptions{excludePatternOptions: excludePatternOptions{InsensitiveExcludes: []string{"*[._]log[.-][0-9]", "!*[._]log[.-][0-9]"}}}, env.gopts)

	rtest.Equals(t, `Fatal: --iexclude: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())
}

func TestBackupFailsWhenUsingInvalidPatternsFromFile(t *testing.T) {
	env, cleanup := withTestEnvironment(t)
	defer cleanup()

	testRunInit(t, env.gopts)

	// Create an exclude file with some invalid patterns
	excludeFile := env.base + "/excludefile"
	fileErr := os.WriteFile(excludeFile, []byte("*.go\n*[._]log[.-][0-9]\n!*[._]log[.-][0-9]"), 0644)
	if fileErr != nil {
		t.Fatalf("Could not write exclude file: %v", fileErr)
	}

	var err error

	// Test --exclude-file:
	err = testRunBackupAssumeFailure(t, filepath.Dir(env.testdata), []string{"testdata"}, BackupOptions{excludePatternOptions: excludePatternOptions{ExcludeFiles: []string{excludeFile}}}, env.gopts)

	rtest.Equals(t, `Fatal: --exclude-file: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())

	// Test --iexclude-file
	err = testRunBackupAssumeFailure(t, filepath.Dir(env.testdata), []string{"testdata"}, BackupOptions{excludePatternOptions: excludePatternOptions{InsensitiveExcludeFiles: []string{excludeFile}}}, env.gopts)

	rtest.Equals(t, `Fatal: --iexclude-file: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())
}

func TestRestoreFailsWhenUsingInvalidPatterns(t *testing.T) {
	env, cleanup := withTestEnvironment(t)
	defer cleanup()

	testRunInit(t, env.gopts)

	var err error

	// Test --exclude
	err = testRunRestoreAssumeFailure("latest", RestoreOptions{excludePatternOptions: excludePatternOptions{Excludes: []string{"*[._]log[.-][0-9]", "!*[._]log[.-][0-9]"}}}, env.gopts)

	rtest.Equals(t, `Fatal: --exclude: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())

	// Test --iexclude
	err = testRunRestoreAssumeFailure("latest", RestoreOptions{excludePatternOptions: excludePatternOptions{InsensitiveExcludes: []string{"*[._]log[.-][0-9]", "!*[._]log[.-][0-9]"}}}, env.gopts)

	rtest.Equals(t, `Fatal: --iexclude: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())

	// Test --include
	err = testRunRestoreAssumeFailure("latest", RestoreOptions{includePatternOptions: includePatternOptions{Includes: []string{"*[._]log[.-][0-9]", "!*[._]log[.-][0-9]"}}}, env.gopts)

	rtest.Equals(t, `Fatal: --include: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())

	// Test --iinclude
	err = testRunRestoreAssumeFailure("latest", RestoreOptions{includePatternOptions: includePatternOptions{InsensitiveIncludes: []string{"*[._]log[.-][0-9]", "!*[._]log[.-][0-9]"}}}, env.gopts)

	rtest.Equals(t, `Fatal: --iinclude: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())
}

func TestRestoreFailsWhenUsingInvalidPatternsFromFile(t *testing.T) {
	env, cleanup := withTestEnvironment(t)
	defer cleanup()

	testRunInit(t, env.gopts)

	// Create an include file with some invalid patterns
	patternsFile := env.base + "/patternsFile"
	fileErr := os.WriteFile(patternsFile, []byte("*.go\n*[._]log[.-][0-9]\n!*[._]log[.-][0-9]"), 0644)
	if fileErr != nil {
		t.Fatalf("Could not write include file: %v", fileErr)
	}

	err := testRunRestoreAssumeFailure("latest", RestoreOptions{includePatternOptions: includePatternOptions{IncludeFiles: []string{patternsFile}}}, env.gopts)
	rtest.Equals(t, `Fatal: --include-file: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())

	err = testRunRestoreAssumeFailure("latest", RestoreOptions{excludePatternOptions: excludePatternOptions{ExcludeFiles: []string{patternsFile}}}, env.gopts)
	rtest.Equals(t, `Fatal: --exclude-file: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())

	err = testRunRestoreAssumeFailure("latest", RestoreOptions{includePatternOptions: includePatternOptions{InsensitiveIncludeFiles: []string{patternsFile}}}, env.gopts)
	rtest.Equals(t, `Fatal: --iinclude-file: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())

	err = testRunRestoreAssumeFailure("latest", RestoreOptions{excludePatternOptions: excludePatternOptions{InsensitiveExcludeFiles: []string{patternsFile}}}, env.gopts)
	rtest.Equals(t, `Fatal: --iexclude-file: invalid pattern(s) provided:
*[._]log[.-][0-9]
!*[._]log[.-][0-9]`, err.Error())
}
