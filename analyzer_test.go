package main

import (
	"fmt"
	"go/ast"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAll(t *testing.T) {
	_ = analyzer.Flags.Set("testrun", "true")
	analysistest.Run(t, analysistest.TestData(), analyzer, "./...")
}

func Test_getLockName(t *testing.T) {
	const lockName = "testLockName"

	testCases := []struct {
		comment               ast.Comment
		expectedLockName      string
		expectedError         error
		ignoreTestWantComment bool
	}{
		{
			comment:          ast.Comment{Text: ""},
			expectedLockName: "",
			expectedError:    fmt.Errorf("found 0 \"protected by \" in comment \"\", expected exact one"),
		},
		{
			comment:          ast.Comment{Text: "protected bytestLockName"},
			expectedLockName: "",
			expectedError:    fmt.Errorf("found 0 \"protected by \" in comment \"protected bytestLockName\", expected exact one"),
		},
		{
			comment:          ast.Comment{Text: "protected by field1, protected by field2"},
			expectedLockName: "",
			expectedError:    fmt.Errorf("found 2 \"protected by \" in comment \"protected by field1, protected by field2\", expected exact one"),
		},
		{
			comment:          ast.Comment{Text: "protected by: testLockName"},
			expectedLockName: "",
			expectedError:    fmt.Errorf("found 0 \"protected by \" in comment \"protected by: testLockName\", expected exact one"),
		},
		{
			comment:          ast.Comment{Text: "protected by testLockName"},
			expectedLockName: lockName,
			expectedError:    nil,
		},
		{
			comment:          ast.Comment{Text: "protected by testLockName. The rest is not important"},
			expectedLockName: lockName,
			expectedError:    nil,
		},
		{
			comment:          ast.Comment{Text: "protected by      testLockName with multiple spaces"},
			expectedLockName: lockName,
			expectedError:    nil,
		},
		{
			comment:          ast.Comment{Text: "// protected by \"testLockName\""},
			expectedLockName: lockName,
			expectedError:    nil,
		},
		{
			comment:          ast.Comment{Text: "// protected by testLockName// want ` protected by foo"},
			expectedLockName: lockName,
			expectedError:    nil,
		},
		{
			comment:          ast.Comment{Text: "// Protected by testLockName."},
			expectedLockName: lockName,
			expectedError:    nil,
		},
	}

	const testRun = true
	for _, tc := range testCases {
		t.Run(tc.comment.Text, func(t *testing.T) {
			name, err := getLockName(&tc.comment, testRun)
			if !errorsEqual(tc.expectedError, err) {
				t.Fatalf("expected error [%s], got [%s]", tc.expectedError, err)
			}

			if name != tc.expectedLockName {
				t.Fatalf("expected lock name %q, got %q", tc.expectedError, err)
			}
		})
	}
}

func errorsEqual(err1, err2 error) bool {
	if err1 == nil && err2 == nil {
		return true
	}

	return maybeErrorMessage(err1) == maybeErrorMessage(err2)
}

func maybeErrorMessage(err error) string {
	if e, ok := err.(*analysisError); ok && e == nil || !ok && err == nil {
		return ""
	}
	return err.Error()
}
