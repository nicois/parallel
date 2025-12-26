package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

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
		if expiry := parallel.GetS3ExpiryTime(); expiry != nil {
			safetyMargin := expiry.Add(-5 * time.Minute)
			if safetyMargin.Before(time.Now()) {
				logger.Error("too close to AWS token expiration", slog.Time("shutdown time", safetyMargin), slog.Time("token expiry time", *expiry), slog.String("duration until safety margin is reached", parallel.FriendlyDuration(time.Until(safetyMargin))))
				os.Exit(1)
			}
			logger.Info("shutting down before the AWS token expires", slog.Time("shutdown time", safetyMargin), slog.Time("token expiry time", *expiry), slog.String("duration until safety margin is reached", parallel.FriendlyDuration(time.Until(safetyMargin))))
			var dCancel context.CancelFunc
			ctx, dCancel = context.WithDeadlineCause(ctx, *expiry, errors.New("AWS token will expire soon"))
			defer dCancel()
		}
	} else {
		cache = parallel.NewFileCache(*opts.CacheLocation)
	}
	err = parallel.PrepareAndRun(ctx, reader, opts, commandLine, cache, interruptChannel)

	// show exit reasons
	if err != nil {
		if err != parallel.ErrUserCancelled {
			logger.Error(fmt.Sprintf("%v", err))
		}
		os.Exit(1)
	}
}
