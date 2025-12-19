package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jessevdk/go-flags"
	"github.com/lmittmann/tint"
	"github.com/nicois/parallel"
)

var logger *slog.Logger

func main() {
	// collect command-line options
	var opts parallel.Opts
	commandLine, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	// set up the logger
	var handler slog.Handler
	handlerOptions := tint.Options{}
	if opts.Debug {
		handlerOptions.Level = slog.LevelDebug
		handlerOptions.AddSource = true
	} else {
		handlerOptions.Level = slog.LevelInfo
	}
	handler = tint.NewHandler(os.Stdout, &handlerOptions)
	logger = slog.New(handler)
	parallel.SetLogger(logger)

	// listen for signals
	// to support escalation, do not simply use NotifyContext
	interruptChannel := make(chan os.Signal, 2)
	signal.Notify(interruptChannel, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// provide stub commands if required
	if len(commandLine) == 0 {
		if opts.CSV || opts.JsonLine {
			commandLine = []string{"echo", "foo is {{.foo}}, bar is {{.bar}}"}
		} else {
			commandLine = []string{"echo", "value is {{.value}}"}
		}
		logger.Info("no command was provided, so just echoing the input", slog.Any("commandline", commandLine))
	}

	// prepare for processing STDIN
	reader := bufio.NewReader(os.Stdin)
	var cache parallel.Cache
	ctx := context.Background()
	if opts.CacheLocation == nil {
		cache = parallel.NewFileCache(filepath.Join(parallel.Must(os.UserHomeDir()), ".cache", "parallel"))
	} else if strings.HasPrefix(*opts.CacheLocation, "s3://") {
		cache, err = parallel.NewS3Cache(ctx, *opts.CacheLocation)
		if err != nil {
			logger.Error("cannot initialise S3 cache", slog.Any("error", err))
			os.Exit(1)
		}
	} else {
		cache = parallel.NewFileCache(*opts.CacheLocation)
	}
	err = parallel.PrepareAndRun(context.Background(), reader, opts, commandLine, cache, interruptChannel)

	// show exit reasons
	if err != nil {
		if err != parallel.ErrUserCancelled {
			logger.Error(fmt.Sprintf("%v", err))
		}
		os.Exit(1)
	}
}
