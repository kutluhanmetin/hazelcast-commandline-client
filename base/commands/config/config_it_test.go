package config_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hazelcast/hazelcast-commandline-client/clc/paths"
	"github.com/hazelcast/hazelcast-commandline-client/internal/check"
	"github.com/hazelcast/hazelcast-commandline-client/internal/it"
)

func TestConfig(t *testing.T) {
	testCases := []struct {
		name string
		f    func(t *testing.T)
	}{
		{name: "Import", f: importTest},
		{name: "Add", f: addTest},
	}
	for _, tc := range testCases {
		t.Run(tc.name, tc.f)
	}
}

func importTest(t *testing.T) {
	tcx := it.TestContext{T: t}
	const configURL = "https://rcd-download.s3.us-east-2.amazonaws.com/hazelcast-cloud-go-sample-client-pr-FOR_TESTING-default.zip"
	tcx.Tester(func(tcx it.TestContext) {
		name := it.NewUniqueObjectName("cfg")
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute("config", "import", name, configURL))
			tcx.AssertStdoutContains("OK\n")
			path := paths.Join(paths.ResolveConfigPath(name))
			tcx.T.Logf("config path: %s", path)
			assert.True(tcx.T, paths.Exists(path))
		})
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute("config", "list"))
			tcx.AssertStdoutContains(name)
		})
	})
}

func addTest(t *testing.T) {
	tcx := it.TestContext{T: t}
	tcx.Tester(func(tcx it.TestContext) {
		name := it.NewUniqueObjectName("cfg")
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute("config", "add", name, "cluster.address=foobar.com"))
			tcx.AssertStdoutContains("OK\n")
		})
		tcx.WithReset(func() {
			check.Must(tcx.CLC().Execute("config", "list"))
			tcx.AssertStdoutContains(name)
		})
		path := paths.ResolveConfigPath(name)
		require.True(tcx.T, paths.Exists(path))
		r := check.MustValue(regexp.Compile(`cluster:\n\s+address: foobar.com`))
		text := check.MustValue(os.ReadFile(path))
		t.Logf(string(text))
		require.True(tcx.T, r.Match(text))
	})
}