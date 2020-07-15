package cmds

import (
	"fmt"
	"reflect"

	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/spur/cli"
	"github.com/rancher/wrangler/pkg/merr"
)

func init() {
	cmds.ConfigFlag.Value = "/etc/rancher/rke2/flags.conf"
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

func flagValue(flag cli.Flag, field string) reflect.Value {
	return reflect.Indirect(reflect.ValueOf(flag)).FieldByName(field)
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
			if v := flagValue(flag, "Usage"); v.IsValid() {
				v.SetString(opt.Usage)
			}
		}
		if opt.Default != "" {
			if v := flagValue(flag, "Default"); v.IsValid() {
				v.SetString(opt.Default)
			}
		}
		if opt.Hide {
			if v := flagValue(flag, "Hidden"); v.IsValid() {
				v.SetBool(true)
			}
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
