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

Features:

- Supports various streaming clients: VLC, ServeStream, ... and most Icecast clients.

- Supports sending of meta data (sending artist and title to the streaming client).

- Playlists are simple json files and data files are normal media (e.g. .mp3) files on disk.

- Supports basic authentication.
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"devt.de/krotik/dudeldu"
	"devt.de/krotik/dudeldu/playlist"
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
	PathPrefix     = "PathPrefix"
)

/*
DefaultConfig is the defaut configuration
*/
var DefaultConfig = map[string]interface{}{
	ThreadPoolSize: 10,
	FrameQueueSize: 10000,
	ServerPort:     "9091",
	ServerHost:     "127.0.0.1",
	PathPrefix:     "",
}

type consolelogger func(v ...interface{})

/*
Fatal/print logger methods. Using a custom type so we can test calls with unit
tests.
*/
var fatal = consolelogger(log.Fatal)
var print = consolelogger(func(a ...interface{}) {
	fmt.Fprint(os.Stderr, a...)
	fmt.Fprint(os.Stderr, "\n")
})

var lookupEnv func(string) (string, bool) = os.LookupEnv

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

	print(fmt.Sprintf("DudelDu %v", dudeldu.ProductVersion))

	auth := flag.String("auth", "", "Authentication as <user>:<pass>")
	serverHost := flag.String("host", DefaultConfig[ServerHost].(string), "Server hostname to listen on")
	serverPort := flag.String("port", DefaultConfig[ServerPort].(string), "Server port to listen on")
	threadPoolSize := flag.Int("tps", DefaultConfig[ThreadPoolSize].(int), "Thread pool size")
	frameQueueSize := flag.Int("fqs", DefaultConfig[FrameQueueSize].(int), "Frame queue size")
	pathPrefix := flag.String("pp", DefaultConfig[PathPrefix].(string), "Prefix all paths with a string")
	enableDebug := flag.Bool("debug", false, "Enable extra debugging output")
	loopPlaylist := flag.Bool("loop", false, "Loop playlists")
	shufflePlaylist := flag.Bool("shuffle", false, "Shuffle playlists")
	showHelp := flag.Bool("?", false, "Show this help message")

	flag.Usage = func() {
		print(fmt.Sprintf("Usage of %s [options] <playlist>", os.Args[0]))
		flag.PrintDefaults()
		print()
		print(fmt.Sprint("Authentication can also be defined via the environment variable: DUDELDU_AUTH=\"<user>:<pass>\""))
	}

	flag.Parse()

	if len(flag.Args()) != 1 || *showHelp {
		flag.Usage()
		return
	}

	// Check for auth environment variable

	if envAuth, ok := lookupEnv("DUDELDU_AUTH"); ok && *auth == "" {
		*auth = envAuth
	}

	laddr := fmt.Sprintf("%v:%v", *serverHost, *serverPort)

	print(fmt.Sprintf("Serving playlist %v on %v", flag.Arg(0), laddr))
	print(fmt.Sprintf("Thread pool size: %v", *threadPoolSize))
	print(fmt.Sprintf("Frame queue size: %v", *frameQueueSize))
	print(fmt.Sprintf("Loop playlist: %v", *loopPlaylist))
	print(fmt.Sprintf("Shuffle playlist: %v", *shufflePlaylist))
	print(fmt.Sprintf("Path prefix: %v", *pathPrefix))
	if *auth != "" {
		print(fmt.Sprintf("Required authentication: %v", *auth))
	}

	// Create server and listen

	plf, err = playlist.NewFilePlaylistFactory(flag.Arg(0), *pathPrefix)

	if err == nil {

		rh := dudeldu.NewDefaultRequestHandler(plf, *loopPlaylist, *shufflePlaylist, *auth)
		dds = dudeldu.NewServer(rh.HandleRequest)
		dds.DebugOutput = *enableDebug

		rh.SetDebugLogger(dds)

		defer print("Shutting down")

		err = dds.Run(laddr, nil)
	}

	if err != nil {
		fatal(err)
	}
}
