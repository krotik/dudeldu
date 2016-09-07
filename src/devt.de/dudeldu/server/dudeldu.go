/*
 * DudelDu
 *
 * Copyright 2016 Matthias Ladkau. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the MIT
 * License, If a copy of the MIT License was not distributed with this
 * file, You can obtain one at https://opensource.org/licenses/MIT.
 */

/*
DudelDu main entry point for the standalone server.
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"devt.de/dudeldu"
	"devt.de/dudeldu/playlist"
	"devt.de/dudeldu/version"
)

// Global variables
// ================

/*
ConfigFile is the config file which will be used to configure DudelDu
*/
var ConfigFile = "dudeldu.config.json"

/*
Known configuration options for DudelDu
*/
const (
	ThreadPoolSize = "ThreadPoolSize"
	FrameQueueSize = "FrameQueueSize"
	ServerPort     = "ServerPort"
	ServerHost     = "ServerHost"
)

/*
DefaultConfig is the defaut configuration
*/
var DefaultConfig = map[string]interface{}{
	ThreadPoolSize: 10,
	FrameQueueSize: 10000,
	ServerPort:     "9091",
	ServerHost:     "localhost",
}

type consolelogger func(v ...interface{})

/*
Fatal/print logger methods. Using a custom type so we can test calls with unit
tests.
*/
var fatal consolelogger = log.Fatal
var print consolelogger = func(a ...interface{}) {
	fmt.Fprint(os.Stderr, a...)
	fmt.Fprint(os.Stderr, "\n")
}

/*
DudelDu server instance (used by unit tests)
*/
var dds *dudeldu.Server

/*
Main entry point for DudelDu.
*/
func main() {
	var err error
	var plf dudeldu.PlaylistFactory

	print(fmt.Sprintf("DudelDu %v.%v", version.VERSION, version.REV))

	serverHost := flag.String("host", DefaultConfig[ServerHost].(string), "Server hostname to listen on")
	serverPort := flag.String("port", DefaultConfig[ServerPort].(string), "Server port to listen on")
	threadPoolSize := flag.Int("tps", DefaultConfig[ThreadPoolSize].(int), "Thread pool size")
	frameQueueSize := flag.Int("fqs", DefaultConfig[FrameQueueSize].(int), "Frame queue size")
	enableDebug := flag.Bool("debug", false, "Enable extra debugging output")
	loopPlaylist := flag.Bool("loop", false, "Loop playlists")
	shufflePlaylist := flag.Bool("shuffle", false, "Shuffle playlists")
	showHelp := flag.Bool("?", false, "Show this help message")

	flag.Usage = func() {
		print(fmt.Sprintf("Usage of %s [options] <playlist>", os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	if len(flag.Args()) != 1 || *showHelp {
		flag.Usage()
		return
	}

	dudeldu.DebugOutput = *enableDebug

	laddr := fmt.Sprintf("%v:%v", *serverHost, *serverPort)

	print(fmt.Sprintf("Serving playlist %v on %v", flag.Arg(0), laddr))
	print(fmt.Sprintf("Thread pool size: %v", *threadPoolSize))
	print(fmt.Sprintf("Frame queue size: %v", *frameQueueSize))
	print(fmt.Sprintf("Loop playlist: %v", *loopPlaylist))
	print(fmt.Sprintf("Shuffle playlist: %v", *shufflePlaylist))

	// Create server and listen

	plf, err = playlist.NewFilePlaylistFactory(flag.Arg(0))

	if err == nil {

		rh := dudeldu.NewDefaultRequestHandler(plf, *loopPlaylist, *shufflePlaylist)
		dds = dudeldu.NewServer(rh.HandleRequest)

		defer print("Shutting down")

		err = dds.Run(laddr, nil)
	}

	if err != nil {
		fatal(err)
	}
}
