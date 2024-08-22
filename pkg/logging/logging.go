package logging

import (
	"flag"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/natefinch/lumberjack.v2"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

var (
	packageFlags  = pflag.NewFlagSet("logging", pflag.ContinueOnError)
	defaultValues = map[string]string{}
)

// init binds klog flags (except v/vmodule) into the pflag flagset and applies normalization from upstream, and
// memoize default values so that they can be reset when reusing the flag parser.
// Refs:
// * https://github.com/kubernetes/kubernetes/blob/release-1.25/staging/src/k8s.io/component-base/logs/logs.go#L49
// * https://github.com/kubernetes/kubernetes/blob/release-1.25/staging/src/k8s.io/component-base/logs/logs.go#L83
func init() {
	packageFlags.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	fs := flag.NewFlagSet("logging", flag.ContinueOnError)
	klog.InitFlags(fs)
	fs.VisitAll(func(f *flag.Flag) {
		if !strings.HasPrefix(f.Name, "v") {
			pf := pflag.PFlagFromGoFlag(f)
			defaultValues[pf.Name] = pf.DefValue
			packageFlags.AddFlag(pf)
		}
	})
}

// ExtractFromArgs extracts the legacy klog flags from an args list, and returns both the remaining args,
// and a similarly configured log writer configured using the klog flag values.
func ExtractFromArgs(args []string) ([]string, io.Writer) {
	// reset values to default
	for name, value := range defaultValues {
		packageFlags.Set(name, value)
	}

	// filter out and set klog flags
	extraArgs := []string{}
	for _, arg := range args {
		name := strings.TrimPrefix(arg, "--")
		split := strings.SplitN(name, "=", 2)
		if flag := packageFlags.Lookup(split[0]); flag != nil {
			var val string
			if len(split) > 1 {
				val = split[1]
			} else {
				val = flag.NoOptDefVal
			}
			flag.Value.Set(val)
			continue
		}
		extraArgs = append(extraArgs, arg)
	}

	// Ignore errors on retrieving flag values; accepting the default is fine
	alsoToStderr, _ := packageFlags.GetBool("alsologtostderr")
	filename, _ := packageFlags.GetString("log-file")
	maxSize, _ := packageFlags.GetUint64("log-file-max-size")
	toStderr, _ := packageFlags.GetBool("logtostderr")

	if filename == "" {
		if toStderr || alsoToStderr {
			return extraArgs, os.Stderr
		}
		return extraArgs, io.Discard
	}

	logger := GetLogger(filename, int(maxSize))

	if alsoToStderr {
		return extraArgs, io.MultiWriter(os.Stderr, logger)
	}

	return extraArgs, logger
}

// GetLogger returns a new io.Writer that writes to the specified file
func GetLogger(filename string, maxSize int) io.Writer {
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    int(maxSize),
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}
}
