package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	klog "k8s.io/klog/v2"
	"k8s.io/utils/set"

	"github.com/seal-io/kubecia/cmd/apis"
	"github.com/seal-io/kubecia/cmd/plugins"
	"github.com/seal-io/kubecia/pkg/signal"
	"github.com/seal-io/kubecia/pkg/version"
)

func init() {
	// Adjust Logger.
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlag(flag.CommandLine.Lookup("v"))
	pflag.CommandLine.AddGoFlag(flag.CommandLine.Lookup("logtostderr"))
	_ = pflag.CommandLine.Set("logtostderr", "true")
}

func main() {
	debugArgs := pflag.Bool(
		"debug-args",
		false,
		"debug arguments, which prints all running arguments, only for development",
	)

	rc := &cobra.Command{
		Use:          "kubecia",
		Short:        `Kubecia is an available the client-go credential (exec) plugin, no Cloud Provider CLI required.`,
		SilenceUsage: true,
		Version:      version.Get(),
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			if !*debugArgs {
				return nil
			}

			if len(os.Args) > 1 && filepath.Base(os.Args[0]) == os.Args[1] {
				c.Printf("%s %s\n\n", os.Args[0], strings.Join(os.Args[2:], " "))
				return nil
			}

			c.Printf("%s\n\n", strings.Join(os.Args, " "))
			return nil
		},
		RunE: func(c *cobra.Command, args []string) error {
			return c.Help()
		},
	}

	// Add Commands.
	plugins.AddCommands(rc)
	apis.AddCommands(rc)

	// Retrieve arguments from environment variables.
	retrieveArguments(rc)

	// Set output.
	rc.SetOut(os.Stdout)

	// Execute.
	if err := rc.ExecuteContext(signal.Context()); err != nil {
		os.Exit(1)
	}
}

func retrieveArguments(rc *cobra.Command) {
	const (
		argPrefix    = "--"
		envKeyPrefix = "KUBECIA_"
	)

	cmd := filepath.Base(os.Args[0])

	var sc *cobra.Command

	for _, c := range rc.Commands() {
		if c.Name() == cmd {
			if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], argPrefix) {
				break
			}

			newArgs := make([]string, len(os.Args)+1)
			newArgs[0] = os.Args[0]
			newArgs[1] = cmd
			copy(newArgs[2:], os.Args[1:])
			os.Args = newArgs

			sc = c

			break
		}
	}

	if len(os.Args) < 2 {
		return
	}

	if sc == nil {
		for _, c := range rc.Commands() {
			if c.Name() == os.Args[1] {
				sc = c
				break
			}
		}
	}

	var (
		envPrefix = envKeyPrefix
		flags     = rc.Flags()
	)

	if sc != nil {
		envPrefix = envPrefix + strings.ToUpper(sc.Name()) + "_"
		flags = sc.Flags()
	}

	igns := set.New("help", "v", "version")
	sets := set.New[string]()

	for _, v := range os.Args[2:] {
		if strings.HasPrefix(v, argPrefix) {
			vs := strings.SplitN(v, "=", 2)
			sets.Insert(vs[0])
		}
	}

	envArgs := make([]string, 0, len(os.Environ())*2)

	for _, v := range os.Environ() {
		if v2 := strings.TrimPrefix(v, envPrefix); v == v2 {
			continue
		} else {
			v = v2
		}

		vs := strings.SplitN(v, "=", 2)
		if len(vs) != 2 {
			continue
		}

		var (
			fn = strings.ReplaceAll(strings.ToLower(vs[0]), "_", "-")
			ek = argPrefix + fn
		)

		if igns.Has(fn) || flags.Lookup(fn) == nil || sets.Has(ek) {
			continue
		}

		ev := vs[1]
		if ev2 := os.ExpandEnv(ev); ev2 != "" && ev != ev2 {
			ev = ev2
		}

		envArgs = append(envArgs, ek, ev)
	}

	if len(envArgs) == 0 {
		return
	}

	newArgs := make([]string, len(os.Args)+len(envArgs))
	copy(newArgs, os.Args)
	copy(newArgs[len(os.Args):], envArgs)
	os.Args = newArgs
}
