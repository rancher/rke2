package cmds

import (
	"fmt"
	"strings"

	"github.com/rancher/wrangler/pkg/merr"
	"github.com/urfave/cli"
)

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

func mustCmdFromK3S(cmd cli.Command, flagOpts map[string]*K3SFlagOption) cli.Command {
	cmd, err := commandFromK3S(cmd, flagOpts)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%#v\n", cmd)
	return cmd
}

func commandFromK3S(cmd cli.Command, flagOpts map[string]*K3SFlagOption) (cli.Command, error) {
	var (
		newFlags []cli.Flag
		seen     = map[string]bool{}
		errs     merr.Errors
	)

	for _, flag := range cmd.Flags {
		name := strings.SplitN(flag.GetName(), ",", 2)[0]
		opt, ok := flagOpts[name]
		if !ok {
			errs = append(errs, fmt.Errorf("new unknown option from k3s %s", flag.GetName()))
			continue
		}
		seen[name] = true
		if opt == nil {
			opt = &K3SFlagOption{}
		}

		if opt.Drop {
			continue
		}

		if strFlag, ok := flag.(cli.StringFlag); ok {
			if opt.Usage != "" {
				strFlag.Usage = opt.Usage
			}
			if opt.Default != "" {
				strFlag.Value = opt.Default
			}
			if opt.Hide {
				strFlag.Hidden = true
			}
			flag = strFlag
		} else if intFlag, ok := flag.(cli.IntFlag); ok {
			if opt.Usage != "" {
				intFlag.Usage = opt.Usage
			}
			if opt.Hide {
				intFlag.Hidden = true
			}
			flag = intFlag
		} else if boolFlag, ok := flag.(cli.BoolFlag); ok {
			if opt.Usage != "" {
				boolFlag.Usage = opt.Usage
			}
			if opt.Hide {
				boolFlag.Hidden = true
			}
			flag = boolFlag
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
