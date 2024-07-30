package main

import (
	"com.github.tunahansezen/tkube/pkg/cmd"
	_ "com.github.tunahansezen/tkube/pkg/cmd/add"
	_ "com.github.tunahansezen/tkube/pkg/cmd/install"
	ostkube "com.github.tunahansezen/tkube/pkg/os"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var version = "TMP_VERSION"

func main() {
	signalChannel := make(chan os.Signal, 2)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-signalChannel
		println(sig)
		fmt.Println("\033[?25h") // make cursor visible
		os.Exit(1)
	}()
	cmd.Execute(version)
	ostkube.Exit("", 0)
}
