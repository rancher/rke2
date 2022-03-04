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
	ignore = &K3SFlagOption{
		Ignore: true,
	}
)
var copy *K3SFlagOption

type K3SFlagOption struct {
	Hide    bool
	Drop    bool
	Ignore  bool
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
			if opt.Hide {
				strFlag.Hidden = true
			}
			flag = strFlag
		} else if strFlag, ok := flag.(*cli.StringFlag); ok {
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
		} else if strSliceFlag, ok := flag.(cli.StringSliceFlag); ok {
			if opt.Usage != "" {
				strSliceFlag.Usage = opt.Usage
			}
			if opt.Default != "" {
				slice := &cli.StringSlice{}
				parts := strings.Split(opt.Default, ",")
				for _, val := range parts {
					slice.Set(val)
				}
				strSliceFlag.Value = slice
			}
			if opt.Hide {
				strSliceFlag.Hidden = true
			}
			flag = strSliceFlag
		} else if intFlag, ok := flag.(cli.IntFlag); ok {
			if opt.Usage != "" {
				intFlag.Usage = opt.Usage
			}
			if opt.Hide {
				intFlag.Hidden = true
			}
			flag = intFlag
		} else if intFlag, ok := flag.(*cli.IntFlag); ok {
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
		} else if boolFlag, ok := flag.(*cli.BoolFlag); ok {
			if opt.Usage != "" {
				boolFlag.Usage = opt.Usage
			}
			if opt.Hide {
				boolFlag.Hidden = true
			}
			flag = boolFlag
		} else if durationFlag, ok := flag.(*cli.DurationFlag); ok {
			if opt.Usage != "" {
				durationFlag.Usage = opt.Usage
			}
			if opt.Hide {
				durationFlag.Hidden = true
			}
			flag = durationFlag
		} else {
			errs = append(errs, fmt.Errorf("unsupported type %T for flag %s", flag, name))
		}

		newFlags = append(newFlags, flag)
	}

	for k, v := range flagOpts {
		if !seen[k] {
			if v != nil && v.Ignore {
				continue
			}
			errs = append(errs, fmt.Errorf("missing k3s option %s", k))
			continue
		}
	}

	cmd.Flags = newFlags
	return cmd, errs.Err()
}
