package cli

import (
	"bytes"
	"testing"

	"github.com/DishanRajapaksha/industrial-cli-kit/contracttest"
)

func TestSharedCommandContract(t *testing.T) {
	contracttest.Baseline(t, func(args ...string) contracttest.Result {
		var out, errOut bytes.Buffer
		code := NewApp(&out, &errOut).Run(args)
		return contracttest.Result{
			Code:   code,
			Stdout: out.String(),
			Stderr: errOut.String(),
		}
	})
}
