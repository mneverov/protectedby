package main

import (
	"fmt"
	"testing"
)

func Test_getLockName(t *testing.T) {
	const lockName = "testLockName"

	testCases := []struct {
		comment          string
		expectedLockName string
		expectedError    error
	}{
		{
			comment:          "",
			expectedLockName: "",
			expectedError:    fmt.Errorf("found 0 \"protected by \" in \"\", expected exact one"),
		},
		{
			comment:          "protected bytestLockName",
			expectedLockName: "",
			expectedError:    fmt.Errorf("found 0 \"protected by \" in \"protected bytestLockName\", expected exact one"),
		},
		{
			comment:          "protected by field1, protected by field2",
			expectedLockName: "",
			expectedError:    fmt.Errorf("found 2 \"protected by \" in \"protected by field1, protected by field2\", expected exact one"),
		},
		{
			comment:          "protected by: testLockName",
			expectedLockName: "",
			expectedError:    fmt.Errorf("found 0 \"protected by \" in \"protected by: testLockName\", expected exact one"),
		},
		{
			comment:          "protected by testLockName",
			expectedLockName: lockName,
			expectedError:    nil,
		},
		{
			comment:          "protected by testLockName. The rest is not important",
			expectedLockName: lockName,
			expectedError:    nil,
		},
		{
			comment:          "protected by      testLockName with multiple spaces",
			expectedLockName: lockName,
			expectedError:    nil,
		},
		{
			comment:          "// protected by \"testLockName\"",
			expectedLockName: lockName,
			expectedError:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.comment, func(t *testing.T) {
			name, err := getLockName(tc.comment)
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
	if err == nil {
		return ""
	}
	return err.Error()
}
