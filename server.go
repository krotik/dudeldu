/*
 * DudelDu
 *
 * Copyright 2016 Matthias Ladkau. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the MIT
 * License, If a copy of the MIT License was not distributed with this
 * file, You can obtain one at https://opensource.org/licenses/MIT.
 */

package dudeldu

import (
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

/*
ProductVersion is the current version of DudelDu
*/
const ProductVersion = "1.1.0"

/*
ConnectionHandler is a function to handle new connections
*/
type ConnectionHandler func(net.Conn, net.Error)

/*
Server data structure
*/
type Server struct {
	Running     bool              // Flag indicating if the server is running
	Handler     ConnectionHandler // Handler function for new  connections
	signalling  chan os.Signal    // Channel for receiving signals
	tcpListener *net.TCPListener  // TCP listener which accepts connections
	serving     bool              // Internal flag indicating if the socket should be served
	wgStatus    *sync.WaitGroup   // Optional wait group which should be notified once the server has started
}

/*
NewServer creates a new DudelDu server.
*/
func NewServer(handler ConnectionHandler) *Server {
	return &Server{
		Running: false,
		Handler: handler,
	}
}

/*
Run starts the DudelDu Server which can be stopped via ^C (Control-C).

laddr should be the local address which should be given to net.Listen.
wgStatus is an optional wait group which will be notified once the server is listening
and once the server has shutdown.

This function will not return unless the server is shutdown.
*/
func (ds *Server) Run(laddr string, wgStatus *sync.WaitGroup) error {

	// Create listener

	listener, err := net.Listen("tcp", laddr)

	if err != nil {
		if wgStatus != nil {
			wgStatus.Done()
		}

		return err
	}

	ds.tcpListener = listener.(*net.TCPListener)
	ds.wgStatus = wgStatus

	// Attach SIGINT handler - on unix and windows this is send
	// when the user presses ^C (Control-C).

	ds.signalling = make(chan os.Signal)
	signal.Notify(ds.signalling, syscall.SIGINT)

	// Put the serve call into a wait group so we can wait until shutdown
	// completed

	var wg sync.WaitGroup
	wg.Add(1)

	// Kick off the serve thread

	go func() {
		defer wg.Done()

		ds.Running = true
		ds.serv()
	}()

	for {

		// Listen for shutdown signal

		if DebugOutput {
			Print("Listen for shutdown signal")
		}

		signal := <-ds.signalling

		if signal == syscall.SIGINT {

			// Shutdown the server

			ds.serving = false

			// Wait until the server has shut down

			wg.Wait()

			ds.Running = false

			break
		}
	}

	if wgStatus != nil {
		wgStatus.Done()
	}

	return nil
}

/*
Shutdown sends a shutdown signal.
*/
func (ds *Server) Shutdown() {
	if ds.serving {
		ds.signalling <- syscall.SIGINT
	}
}

/*
serv waits for new connections and assigns a handler to them.
*/
func (ds *Server) serv() {

	ds.serving = true

	for ds.serving {

		// Wait up to a second for a new connection

		ds.tcpListener.SetDeadline(time.Now().Add(time.Second))
		newConn, err := ds.tcpListener.Accept()

		// Notify wgStatus if it was specified

		if ds.wgStatus != nil {
			ds.wgStatus.Done()
			ds.wgStatus = nil
		}

		netErr, ok := err.(net.Error)

		// Check if got an error and notify an error handler

		if newConn != nil || (ok && !(netErr.Timeout() || netErr.Temporary())) {

			go ds.Handler(newConn, netErr)
		}
	}

	ds.tcpListener.Close()
}
