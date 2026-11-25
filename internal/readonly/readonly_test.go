package readonly

import (
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsTruthy(t *testing.T) {
	assert.True(t, isTruthy("1"))
	assert.True(t, isTruthy("true"))
	assert.True(t, isTruthy("TRUE"))
	assert.True(t, isTruthy("yes"))
	assert.False(t, isTruthy(""))
	assert.False(t, isTruthy("0"))
	assert.False(t, isTruthy("false"))
}

func TestActive_FromCurrentEnv(t *testing.T) {
	t.Setenv(EnvVar, "1")
	assert.True(t, Active())
}

func TestActive_AncestorEnvCannotBeUnsetOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux /proc ancestor lock test")
	}
	if os.Getenv("TELE_TEST_CHILD") == "1" {
		if !Active() {
			t.Fatal("expected Active() to stay true when ancestor sets TELE_READ_ONLY=1")
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestActive_AncestorEnvCannotBeUnsetOnLinux$")
	// Parent env sets TELE_READ_ONLY=1; spawn an inner sh (without exec, so sh stays as an ancestor)
	// that unsets it before running the child test binary.
	sh := exec.Command("sh", "-c", `unset TELE_READ_ONLY; TELE_READ_ONLY=0 TELE_TEST_CHILD=1 "$@"`, "_", cmd.Path, "-test.run=^TestActive_AncestorEnvCannotBeUnsetOnLinux$")
	sh.Env = append(os.Environ(), EnvVar+"=1")
	out, err := sh.CombinedOutput()
	require.NoError(t, err, "child output:\n%s", string(out))
}
