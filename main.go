package main

import (
	"fmt"
	"os"

	"github.com/jaxxstorm/thresher/cmd"

	log "github.com/jaxxstorm/log"
)

func main() {
	if err := cmd.Execute(); err != nil {
		logger, logErr := log.New(log.Config{Level: log.ErrorLevel, Output: os.Stderr})
		if logErr == nil {
			logger.Error("command failed", log.Error(err))
			_ = logger.Close()
		} else {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}

		os.Exit(1)
	}
}
