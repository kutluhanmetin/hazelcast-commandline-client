/*
 * Copyright (c) 2008-2023, Hazelcast, Inc. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License")
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package it

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	hz "github.com/hazelcast/hazelcast-go-client"
	"gopkg.in/yaml.v2"

	"github.com/hazelcast/hazelcast-commandline-client/clc"
	"github.com/hazelcast/hazelcast-commandline-client/clc/cmd"
	"github.com/hazelcast/hazelcast-commandline-client/clc/config"
	"github.com/hazelcast/hazelcast-commandline-client/clc/paths"
	"github.com/hazelcast/hazelcast-commandline-client/internal/check"
	"github.com/hazelcast/hazelcast-commandline-client/internal/it/expect"
)

const (
	EnvDefaultTimeout = "DEFAULT_TIMEOUT"
	DefaultDelay      = 10 * time.Millisecond
)

type TestContext struct {
	T              *testing.T
	Cluster        *TestCluster
	Client         *hz.Client
	ClientConfig   *hz.Config
	ConfigCallback func(testContext TestContext)
	Before         func(tcx TestContext)
	After          func(tcx TestContext)
	ConfigPath     string
	LogPath        string
	LogLevel       string
	ExpectStdout   *expect.Expect
	ExpectStderr   *expect.Expect
	homePath       string
	stderr         *bytes.Buffer
	stdout         *bytes.Buffer
	stdinR         io.Reader
	stdinW         io.Writer
	main           *cmd.Main
}

func (tcx TestContext) HomePath() string {
	return tcx.homePath
}

func (tcx TestContext) Stderr() *bytes.Buffer {
	return tcx.stderr
}

func (tcx TestContext) Stdout() *bytes.Buffer {
	return tcx.stdout
}

func (tcx TestContext) Stdin() io.Reader {
	return tcx.stdinR
}

func (tcx TestContext) CLC() *cmd.Main {
	return tcx.main
}

func (tcx TestContext) ReadStdout() []byte {
	return check.MustValue(io.ReadAll(tcx.stdout))
}

func (tcx TestContext) ReadStderr() []byte {
	return check.MustValue(io.ReadAll(tcx.stderr))
}

func (tcx TestContext) WriteStdin(b []byte) {
	if _, err := tcx.stdinW.Write(b); err != nil {
		panic(fmt.Errorf("writing to stdin: %w", err))
	}
}

func (tcx TestContext) WriteStdinString(s string) {
	tcx.WriteStdin([]byte(s))
}

func (tcx TestContext) WriteStdinF(format string, args ...any) {
	tcx.WriteStdin([]byte(fmt.Sprintf(format, args...)))
}

func (tcx TestContext) Tester(f func(tcx TestContext)) {
	ensureRemoteController(true)
	runner := func(tcx TestContext) {
		if tcx.Cluster == nil {
			tcx.Cluster = defaultTestCluster.Launch(tcx.T)
		}
		if tcx.ClientConfig == nil {
			cfg := tcx.Cluster.DefaultConfig()
			tcx.ClientConfig = &cfg
		}
		if tcx.ConfigCallback != nil {
			tcx.ConfigCallback(tcx)
		}
		cfg := ConfigToMap(*tcx.ClientConfig)
		bytesConfig, err := yaml.Marshal(cfg)
		if err == nil {
			// note that checking whether there's no error.
			tcx.T.Logf("Config:\n%s", string(bytesConfig))
		}
		addrs := tcx.ClientConfig.Cluster.Network.Addresses
		if len(addrs) > 0 {
			tcx.T.Logf("cluster address: %s", addrs[0])
		}
		home := check.MustValue(NewCLCHome())
		defer home.Destroy()
		if tcx.Client == nil {
			tcx.Client = getDefaultClient(tcx.ClientConfig)
		}
		defer func() {
			ctx := context.Background()
			if err := tcx.Client.Shutdown(ctx); err != nil {
				tcx.T.Logf("Test warning, client not shutdown: %s", err.Error())
			}
		}()
		tcx.ConfigPath = "test-cfg"
		tcx.stderr = &bytes.Buffer{}
		tcx.stdout = &bytes.Buffer{}
		tcx.stdinR, tcx.stdinW = io.Pipe()
		tcx.homePath = home.Path()
		tcx.ExpectStdout = expect.New(tcx.stdout)
		defer tcx.ExpectStdout.Stop()
		tcx.ExpectStderr = expect.New(tcx.stderr)
		defer tcx.ExpectStderr.Stop()
		WithEnv(paths.EnvCLCHome, tcx.homePath, func() {
			WithEnv(clc.EnvMaxCols, "50", func() {
				p := paths.ResolveConfigPath(tcx.ConfigPath)
				d, _ := filepath.Split(p)
				check.Must(os.MkdirAll(d, 0700))
				home.WithFile(p, bytesConfig, func(_ string) {
					fp, err := config.NewFileProvider(tcx.ConfigPath)
					if err != nil {
						panic(err)
					}
					tcx.main = check.MustValue(cmd.NewMain("clc", tcx.ConfigPath, fp, tcx.LogPath, tcx.LogLevel, tcx.IO()))
					defer tcx.main.Exit()
					f(tcx)
				})
			})
		})
	}
	if tcx.Before != nil {
		tcx.Before(tcx)
	}
	if tcx.After != nil {
		defer tcx.After(tcx)
	}
	runner(tcx)
}

func (tcx TestContext) IO() clc.IO {
	return clc.IO{
		Stdin:  tcx.Stdin(),
		Stderr: tcx.Stderr(),
		Stdout: tcx.Stdout(),
	}
}

func (tcx TestContext) AssertStdoutEquals(text string) {
	if !tcx.ExpectStdout.Match(expect.Exact(text), expect.WithTimeout(DefaultTimeout()), expect.WithDelay(DefaultDelay)) {
		tcx.T.Log("STDOUT:", tcx.ExpectStdout.String())
		tcx.T.Fatalf("expect failed, no match for: %s", text)
	}
}

func (tcx TestContext) AssertStderrEquals(text string) {
	if !tcx.ExpectStderr.Match(expect.Exact(text), expect.WithTimeout(DefaultTimeout()), expect.WithDelay(DefaultDelay)) {
		tcx.T.Log("STDERR:", tcx.ExpectStderr.String())
		tcx.T.Fatalf("expect failed, no match for: %s", text)
	}
}

func (tcx TestContext) AssertStderrContains(text string) {
	if !tcx.ExpectStderr.Match(expect.Contains(text), expect.WithTimeout(DefaultTimeout()), expect.WithDelay(DefaultDelay)) {
		tcx.T.Log("STDERR:", tcx.ExpectStderr.String())
		tcx.T.Fatalf("expect failed, no match for: %s", text)
	}
}

func (tcx TestContext) AssertStderrNotContains(text string) {
	if tcx.ExpectStderr.Match(expect.Contains(text), expect.WithTimeout(DefaultTimeout()), expect.WithDelay(DefaultDelay)) {
		tcx.T.Log("STDERR:", tcx.ExpectStderr.String())
		tcx.T.Fatalf("expect failed, matched: %s", text)
	}
}

func (tcx TestContext) AssertStdoutContains(text string) {
	if !tcx.ExpectStdout.Match(expect.Contains(text), expect.WithTimeout(DefaultTimeout())) {
		tcx.T.Log("STDOUT:", tcx.ExpectStdout.String())
		tcx.T.Fatalf("expect failed, no match for: %s", text)
	}
}

func (tcx TestContext) AssertStdoutContainsWithPath(path string) {
	p := string(check.MustValue(os.ReadFile(path)))
	tcx.AssertStdoutContains(p)
}

func (tcx TestContext) AssertStdoutDollar(text string) {
	if !tcx.ExpectStdout.Match(expect.Dollar(text), expect.WithTimeout(DefaultTimeout())) {
		tcx.T.Log("STDOUT:", tcx.ExpectStdout.String())
		tcx.T.Fatalf("expect failed, no match for: %s", text)
	}
}

func (tcx TestContext) AssertStdoutDollarWithPath(path string) {
	p := string(check.MustValue(os.ReadFile(path)))
	tcx.AssertStdoutDollar(p)
}

func (tcx TestContext) AssertStdoutEqualsWithPath(path string) {
	p := string(check.MustValue(os.ReadFile(path)))
	p = strings.ReplaceAll(p, "$", "")
	tcx.AssertStdoutEquals(p)
}

func (tcx TestContext) WithReset(f func()) {
	tcx.ExpectStdout.Reset()
	tcx.ExpectStderr.Reset()
	tcx.stdout.Reset()
	tcx.stderr.Reset()
	f()
}

func (tcx TestContext) CLCExecute(args ...string) {
	a := []string{"-c", tcx.ConfigPath}
	a = append(a, args...)
	check.Must(tcx.CLC().Execute(a...))
}

func (tcx TestContext) WithShell(f func(tcx TestContext)) {
	go func() {
		tcx.CLCExecute()
	}()
	// best effort to exit the shell
	defer tcx.WriteStdin([]byte("\\exit\n"))
	f(tcx)
}

func WithEnv(name, value string, f func()) {
	b, ok := os.LookupEnv(name)
	if ok {
		// error is ignored
		defer os.Setenv(name, b)
	} else {
		// error is ignored
		defer os.Unsetenv(name)
	}
	check.Must(os.Setenv(name, value))
	f()
}

func DefaultTimeout() time.Duration {
	s := os.Getenv(EnvDefaultTimeout)
	d, err := time.ParseDuration(s)
	if err != nil {
		return 1 * time.Second
	}
	return d
}