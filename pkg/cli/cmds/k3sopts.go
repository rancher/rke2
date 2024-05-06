package cmds

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/urfave/cli"
)

var (
	copyFlag   = &K3SFlagOption{}
	dropFlag   = &K3SFlagOption{Drop: true}
	hideFlag   = &K3SFlagOption{Hide: true}
	ignoreFlag = &K3SFlagOption{Ignore: true}
)

// K3SFlagOption describes how a CLI flag from K3s should be wrapped.
type K3SFlagOption struct {
	Hide    bool
	Drop    bool
	Ignore  bool
	Usage   string
	Default string
}

// K3SFlagSet is a map of flag names to options
type K3SFlagSet map[string]*K3SFlagOption

// CopyInto copies flags from a source set into the destination
func (k K3SFlagSet) CopyInto(d K3SFlagSet) {
	for key, val := range k {
		d[key] = val
	}
}

func mustCmdFromK3S(cmd cli.Command, flagOpts K3SFlagSet) cli.Command {
	cmd, err := commandFromK3S(cmd, flagOpts)
	if err != nil {
		panic(errors.Wrapf(err, "failed to wrap command %q", cmd.Name))
	}
	return cmd
}

func commandFromK3S(cmd cli.Command, flagOpts K3SFlagSet) (cli.Command, error) {
	var (
		newFlags []cli.Flag
		seen     = map[string]bool{}
		errs     merr.Errors
	)

	for _, flag := range cmd.Flags {
		name := parseName(flag)
		opt, ok := flagOpts[name]
		if !ok {
			errs = append(errs, fmt.Errorf("missing flag options for k3s flag %q", name))
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
		} else if strSliceFlag, ok := flag.(*cli.StringSliceFlag); ok {
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
		} else if boolTFlag, ok := flag.(cli.BoolTFlag); ok {
			if opt.Usage != "" {
				boolTFlag.Usage = opt.Usage
			}
			if opt.Hide {
				boolTFlag.Hidden = true
			}
			flag = boolTFlag
		} else if boolTFlag, ok := flag.(*cli.BoolTFlag); ok {
			if opt.Usage != "" {
				boolTFlag.Usage = opt.Usage
			}
			if opt.Hide {
				boolTFlag.Hidden = true
			}
			flag = boolTFlag
		} else if durationFlag, ok := flag.(*cli.DurationFlag); ok {
			if opt.Usage != "" {
				durationFlag.Usage = opt.Usage
			}
			if opt.Hide {
				durationFlag.Hidden = true
			}
			flag = durationFlag
		} else {
			errs = append(errs, fmt.Errorf("unsupported type %T for flag %q", flag, name))
		}

		newFlags = append(newFlags, flag)
	}

	for k, v := range flagOpts {
		if !seen[k] {
			if v != nil && v.Ignore {
				continue
			}
			errs = append(errs, fmt.Errorf("got flag options for unknown k3s flag %q", k))
			continue
		}
	}

	cmd.Flags = newFlags
	return cmd, errs.Err()
}

// parseName returns a single primary flag name.
// Flags with short versions may be named "flag", "flag,f", "f,flag", "flag, f" and so on.
// This returns just the first long version, or the short version if no long is available.
func parseName(flag cli.Flag) string {
	names := strings.Split(flag.GetName(), ",")
	for _, n := range names {
		n = strings.TrimSpace(n)
		if len(n) > 1 {
			return n
		}
	}
	return strings.TrimSpace(names[0])
}
