package exec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/longhorn/go-common-libs/types"
)

func TestExecute(t *testing.T) {
	type testCase struct {
		command []string
		timeout time.Duration

		expected            string
		expectedErrorPrefix string
	}
	testCases := map[string]testCase{
		"Valid command": {
			command:  []string{"echo", "hello"},
			timeout:  types.ExecuteNoTimeout,
			expected: "hello\n",
		},
		"With error": {
			command:             []string{"ls", "/not-exist"},
			timeout:             types.ExecuteNoTimeout,
			expectedErrorPrefix: "failed to execute",
		},
		"With timeout": {
			command: []string{"sleep", "1"},
			timeout: 2 * time.Second,
		},
		"With timeout and error": {
			command:             []string{"sleep", "1"},
			timeout:             time.Nanosecond,
			expectedErrorPrefix: "timeout executing",
		},
	}
	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			executor := NewExecutor()
			output, err := executor.Execute(nil, testCase.command[0], testCase.command[1:], testCase.timeout)
			if testCase.expectedErrorPrefix != "" {
				assert.Error(t, err)
				assert.True(t, strings.HasPrefix(err.Error(), testCase.expectedErrorPrefix))
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, output)

		})
	}
}

func TestExecuteWithStdin(t *testing.T) {
	type testCase struct {
		commandStdin string
		timeout      time.Duration

		expected    string
		expectError bool
	}
	testCases := map[string]testCase{
		"Echo stdin input": {
			commandStdin: "foo",
			expected:     "foo\n",
		},
	}
	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			if testCase.timeout == 0 {
				testCase.timeout = types.ExecuteDefaultTimeout
			}

			executor := NewExecutor()

			binary := "bash"
			args := []string{"-c", "read input; echo ${input}"}
			output, err := executor.ExecuteWithStdin(binary, args, testCase.commandStdin, testCase.timeout)
			if testCase.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, output)

		})
	}
}

// TestExecuteTimeoutKillsChild proves that a command whose timeout fires is
// actually killed, rather than being orphaned to run to completion after the
// Go caller has already returned. The marker file is written only if the
// command runs to completion; with the kill-on-timeout fix it must never be
// created. (Before the fix the orphaned shell would wake up after its sleep
// and write the marker, failing this assertion.)
func TestExecuteTimeoutKillsChild(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), "completed")

	// `sleep 1` then write the marker. The 100ms timeout must fire first and
	// SIGKILL the shell before it ever reaches the echo.
	executor := NewExecutor()
	_, err := executor.Execute(nil, "sh", []string{"-c", "sleep 1; echo done > " + markerPath}, 100*time.Millisecond)

	assert.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "timeout executing"))

	// Give an orphaned shell (had the kill not happened) enough time to finish
	// its sleep and write the marker, then assert it never did.
	time.Sleep(1500 * time.Millisecond)
	_, statErr := os.Stat(markerPath)
	assert.True(t, os.IsNotExist(statErr), "timeout-killed command wrote its completion marker; child was not killed")
}

func TestExecuteWithStdinPipe(t *testing.T) {
	type testCase struct {
		command      []string
		commandStdin string
		timeout      time.Duration

		expected    string
		expectError bool
	}
	testCases := map[string]testCase{
		"Counts stdin bytes using wc": {
			command:      []string{"wc", "-c"},
			commandStdin: "count me",
			expected:     "8\n",
		},
		"Command times out": {
			command:      []string{"sleep", "1"},
			commandStdin: "ignore me",
			timeout:      time.Nanosecond,
			expectError:  true,
		},
	}
	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			if testCase.timeout == 0 {
				testCase.timeout = types.ExecuteDefaultTimeout
			}

			executor := NewExecutor()

			output, err := executor.ExecuteWithStdinPipe(testCase.command[0], testCase.command[1:], testCase.commandStdin, testCase.timeout)
			if testCase.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			assert.Equal(t, testCase.expected, output)

		})
	}
}
