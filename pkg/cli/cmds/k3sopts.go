package cmds

import (
	"fmt"
	"strings"

	"github.com/rancher/wrangler/pkg/merr"
	"github.com/urfave/cli"
)

var (
	Drop = &K3SFlagOption{
		Drop: true,
	}
	Hide = &K3SFlagOption{
		Hide: true,
	}
)

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
			if strFlag.EnvVar != "" {
				strFlag.EnvVar = strings.Replace(strFlag.EnvVar, "K3S_", "RKE2_", 1)
			}
			strFlag.Usage = strings.Replace(strFlag.Usage, "k3s", "RKE2", -1)
			if opt.Hide {
				strFlag.Hidden = true
			}
			flag = strFlag
		} else if intFlag, ok := flag.(cli.IntFlag); ok {
			if opt.Usage != "" {
				intFlag.Usage = opt.Usage
			}
			if intFlag.EnvVar != "" {
				intFlag.EnvVar = strings.Replace(intFlag.EnvVar, "K3S_", "RKE2_", 1)
			}
			intFlag.Usage = strings.Replace(intFlag.Usage, "k3s", "RKE2", -1)
			if opt.Hide {
				intFlag.Hidden = true
			}
			flag = intFlag
		} else if boolFlag, ok := flag.(cli.BoolFlag); ok {
			if opt.Usage != "" {
				boolFlag.Usage = opt.Usage
			}
			if boolFlag.EnvVar != "" {
				boolFlag.EnvVar = strings.Replace(boolFlag.EnvVar, "K3S_", "RKE2_", 1)
			}
			boolFlag.Usage = strings.Replace(boolFlag.Usage, "k3s", "RKE2", -1)
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
