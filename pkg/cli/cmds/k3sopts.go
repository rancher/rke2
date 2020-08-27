package cmds

import (
	"fmt"
	"reflect"

	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/spur/cli"
	"github.com/rancher/wrangler/pkg/merr"
)

func init() {
	cmds.ConfigFlag.Value = "/etc/rancher/rke2/config.yaml"
}

var (
	drop = &K3SFlagOption{
		Drop: true,
	}
	hide = &K3SFlagOption{
		Hide: true,
	}
)
var copy *K3SFlagOption = nil

type K3SFlagOption struct {
	Hide    bool
	Drop    bool
	Usage   string
	Default string
}

func mustCmdFromK3S(cmd *cli.Command, flagOpts map[string]*K3SFlagOption) *cli.Command {
	cmd, err := commandFromK3S(cmd, flagOpts)
	if err != nil {
		panic(err)
	}
	return cmd
}

func commandFromK3S(cmd *cli.Command, flagOpts map[string]*K3SFlagOption) (*cli.Command, error) {
	var (
		newFlags []cli.Flag
		seen     = map[string]bool{}
		errs     merr.Errors
	)

	for _, flag := range cmd.Flags {
		name := cli.FlagNames(flag)[0]
		opt, ok := flagOpts[name]
		if !ok {
			errs = append(errs, fmt.Errorf("new unknown option from k3s %s", name))
			continue
		}
		seen[name] = true
		if opt == nil {
			opt = &K3SFlagOption{}
		}

		if opt.Drop {
			continue
		}

		if opt.Usage != "" {
			flagSetUsage(flag, opt)
		}
		if opt.Default != "" {
			flagSetDefault(flag, opt)
		}
		if opt.Hide {
			flagSetHide(flag, opt)
		}
		newFlags = append(newFlags, flag)
	}

	for k := range flagOpts {
		if !seen[k] {
			errs = append(errs, fmt.Errorf("missing k3s option %s", k))
			continue
		}
	}

	cmd.Flags = newFlags
	return cmd, errs.Err()
}

// flagSetUsage receives a flag and a K3S flag option, parses
// both and sets the necessary fields based on the underlying
// flag type.
func flagSetUsage(flag cli.Flag, opt *K3SFlagOption) {
	v := reflect.ValueOf(flag).Elem()
	if v.CanSet() {
		switch t := flag.(type) {
		case *cli.StringFlag:
			t.Usage = opt.Usage
		case *cli.StringSliceFlag:
			t.Usage = opt.Usage
		case *cli.BoolFlag:
			t.Usage = opt.Usage
		}
	}
}

// flagSetDefault receives a flag and a K3S flag option, parses
// both and sets the necessary fields based on the underlying
// flag type.
func flagSetDefault(flag cli.Flag, opt *K3SFlagOption) {
	v := reflect.ValueOf(flag).Elem()
	if v.CanSet() {
		switch t := flag.(type) {
		case *cli.StringFlag:
			t.DefaultText = opt.Default
			t.Destination = &opt.Default
			t.Value = opt.Default
		case *cli.StringSliceFlag:
			t.DefaultText = opt.Default
			t.Destination = &([]string{opt.Default})
			t.Value = []string{opt.Default}
		case *cli.BoolFlag:
			t.DefaultText = opt.Default
			t.Destination = &opt.Hide
			t.Value = opt.Hide
		}
	}
}

// flagSetHide receives a flag and a K3S flag option, parses
// both and sets the necessary fields based on the underlying
// flag type.
func flagSetHide(flag cli.Flag, opt *K3SFlagOption) {
	v := reflect.ValueOf(flag).Elem()
	if v.CanSet() {
		switch t := flag.(type) {
		case *cli.StringFlag:
			t.Hidden = opt.Hide
		case *cli.StringSliceFlag:
			t.Hidden = opt.Hide
		case *cli.BoolFlag:
			t.Hidden = opt.Hide
		}
	}
}
