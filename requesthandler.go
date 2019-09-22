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
	"bytes"
	"fmt"
	"io"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"

	"devt.de/krotik/common/datautil"
)

/*
MaxRequestSize is the maximum size for a request
*/
const MaxRequestSize = 1024

/*
MetaDataInterval is the data interval in which meta data is send
*/
var MetaDataInterval uint64 = 65536

/*
peerNoAuthTimeout is the time in seconds a peer can open new connections without
sending new authentication information.
*/
const peerNoAuthTimeout = 10

/*
MaxMetaDataSize is the maximum size for meta data (everything over is truncated)

Must be a multiple of 16 which fits into one byte. Maximum: 16 * 255 = 4080
*/
var MaxMetaDataSize = 4080

/*
requestPathPattern is the pattern which is used to extract the requested path
(i case-insensitive / m multi-line mode: ^ and $ match begin/end line)
*/
var requestPathPattern = regexp.MustCompile("(?im)get\\s+([^\\s]+).*")

/*
requestOffsetPattern is the pattern which is used to extract the requested offset
(i case-insensitive / m multi-line mode: ^ and $ match begin/end line)
*/
var requestOffsetPattern = regexp.MustCompile("(?im)^Range: bytes=([0-9]+)-.*$")

/*
DefaultRequestHandler data structure
*/
type DefaultRequestHandler struct {
	PlaylistFactory PlaylistFactory // Factory for playlists
	ServeRequest    func(c net.Conn, path string,
		metaDataSupport bool, offset int, auth string) // Function to serve requests
	loop      bool               // Flag if the playlist should be looped
	LoopTimes int                // Number of loops -1 loops forever
	shuffle   bool               // Flag if the playlist should be shuffled
	auth      string             // Required (basic) authentication string - may be empty
	authPeers *datautil.MapCache // Peers which have been authenticated
	logger    DebugLogger        // Logger for debug output
}

/*
NewDefaultRequestHandler creates a new default request handler object.
*/
func NewDefaultRequestHandler(pf PlaylistFactory, loop bool,
	shuffle bool, auth string) *DefaultRequestHandler {

	drh := &DefaultRequestHandler{
		PlaylistFactory: pf,
		loop:            loop,
		LoopTimes:       -1,
		shuffle:         shuffle,
		auth:            auth,
		authPeers:       datautil.NewMapCache(0, peerNoAuthTimeout),
		logger:          nil,
	}
	drh.ServeRequest = drh.defaultServeRequest
	return drh
}

/*
SetDebugLogger sets the debug logger for this request handler.
*/
func (drh *DefaultRequestHandler) SetDebugLogger(logger DebugLogger) {
	drh.logger = logger
}

/*
HandleRequest handles requests from streaming clients. It tries to extract
the path and if meta data is supported. Once a request has been successfully
decoded ServeRequest is called. The connection is closed once HandleRequest
finishes.
*/
func (drh *DefaultRequestHandler) HandleRequest(c net.Conn, nerr net.Error) {

	drh.logger.PrintDebug("Handling request from: ", c.RemoteAddr())

	defer func() {
		c.Close()
	}()

	// Check if there was an error

	if nerr != nil {
		drh.logger.PrintDebug(nerr)
		return
	}

	buf, err := drh.decodeRequestHeader(c)
	if err != nil {
		drh.logger.PrintDebug(err)
		return
	}

	// Add ending sequence in case the client "forgets"

	bufStr := buf.String() + "\r\n\r\n"

	// Determine the remote string

	clientString := "-"
	if c.RemoteAddr() != nil {
		clientString, _, _ = net.SplitHostPort(c.RemoteAddr().String())
	}

	drh.logger.PrintDebug("Client:", c.RemoteAddr(), " Request:", bufStr)

	if i := strings.Index(bufStr, "\r\n\r\n"); i >= 0 {
		var auth string
		var ok bool

		bufStr = strings.TrimSpace(bufStr[:i])

		// Check authentication

		if auth, bufStr, ok = drh.checkAuth(bufStr, clientString); !ok {
			drh.writeUnauthorized(c)
			return
		}

		// Check if the client supports meta data

		metaDataSupport := false

		if strings.Contains(strings.ToLower(bufStr), "icy-metadata: 1") {
			metaDataSupport = true
		}

		// Extract offset

		offset := 0
		res := requestOffsetPattern.FindStringSubmatch(bufStr)

		if len(res) > 1 {

			if o, err := strconv.Atoi(res[1]); err == nil {
				offset = o
			}
		}

		// Extract the path

		res = requestPathPattern.FindStringSubmatch(bufStr)

		if len(res) > 1 {

			// Now serve the request

			drh.ServeRequest(c, res[1], metaDataSupport, offset, auth)

			return
		}
	}

	drh.logger.PrintDebug("Invalid request: ", bufStr)
}

/*
decodeRequestHeader decodes the header of an incoming request.
*/
func (drh *DefaultRequestHandler) decodeRequestHeader(c net.Conn) (*bytes.Buffer, error) {
	var buf bytes.Buffer

	rbuf := make([]byte, 512, 512)

	// Decode request

	n, err := c.Read(rbuf)

	for n > 0 || err != nil && err != io.EOF {

		// Do some error checking

		if err != nil {
			return nil, err
		} else if buf.Len() > MaxRequestSize {
			return nil, fmt.Errorf("Illegal request: Request is too long")
		}

		buf.Write(rbuf[:n])

		if strings.Contains(string(rbuf), "\r\n\r\n") {
			break
		}

		n, err = c.Read(rbuf)
	}

	return &buf, nil
}

/*
defaultServeRequest is called once a request was successfully decoded.
*/
func (drh *DefaultRequestHandler) defaultServeRequest(c net.Conn, path string, metaDataSupport bool, offset int, auth string) {
	var writtenBytes uint64
	var currentPlaying string
	var err error

	drh.logger.PrintDebug("Serve request path:", path, " Metadata support:", metaDataSupport, " Offset:", offset)

	pl := drh.PlaylistFactory.Playlist(path, drh.shuffle)
	if pl == nil {

		// Stream was not found - no error checking here (don't care)

		drh.writeStreamNotFoundResponse(c)
		return
	}

	err = drh.writeStreamStartResponse(c, pl.Name(), pl.ContentType(), metaDataSupport)

	frameOffset := offset

	for {
		for !pl.Finished() {

			if drh.logger.IsDebugOutputEnabled() {
				playingString := fmt.Sprintf("%v - %v", pl.Title(), pl.Artist())

				if playingString != currentPlaying {
					currentPlaying = playingString
					drh.logger.PrintDebug("Written bytes: ", writtenBytes)
					drh.logger.PrintDebug("Sending: ", currentPlaying)
				}
			}

			// Check if there were any errors

			if err != nil {
				drh.logger.PrintDebug(err)
				return
			}

			frameOffset, writtenBytes, err = drh.writeFrame(c, pl, frameOffset,
				writtenBytes, metaDataSupport)
		}

		// Handle looping - do not loop if close returns an error

		if pl.Close() != nil || !drh.loop {
			break
		} else if drh.LoopTimes != -1 {
			drh.LoopTimes--
			if drh.LoopTimes == 0 {
				break
			}
		}
	}

	drh.logger.PrintDebug("Serve request path:", path, " complete")
}

/*
prepareFrame prepares a frame before it can be written to a client.
*/
func (drh *DefaultRequestHandler) prepareFrame(c net.Conn, pl Playlist, frameOffset int,
	writtenBytes uint64, metaDataSupport bool) ([]byte, int, error) {

	frame, err := pl.Frame()

	// Handle offsets

	if frameOffset > 0 && err == nil {

		for frameOffset > len(frame) && err == nil {
			frameOffset -= len(frame)
			frame, err = pl.Frame()
		}

		if err == nil {
			frame = frame[frameOffset:]
			frameOffset = 0

			if len(frame) == 0 {
				frame, err = pl.Frame()
			}
		}
	}

	if frame == nil {

		if !pl.Finished() {
			drh.logger.PrintDebug(fmt.Sprintf("Empty frame for: %v - %v (Error: %v)", pl.Title(), pl.Artist(), err))
		}

	} else if err != nil {

		if err != ErrPlaylistEnd {
			drh.logger.PrintDebug(fmt.Sprintf("Error while retrieving playlist data: %v", err))
		}

		err = nil
	}

	return frame, frameOffset, err
}

/*
writeFrame writes a frame to a client.
*/
func (drh *DefaultRequestHandler) writeFrame(c net.Conn, pl Playlist, frameOffset int,
	writtenBytes uint64, metaDataSupport bool) (int, uint64, error) {

	frame, frameOffset, err := drh.prepareFrame(c, pl, frameOffset, writtenBytes, metaDataSupport)
	if frame == nil {
		return frameOffset, writtenBytes, err
	}

	// Check if meta data should be send

	if metaDataSupport && writtenBytes+uint64(len(frame)) >= MetaDataInterval {

		// Write rest data before sending meta data

		if preMetaDataLength := MetaDataInterval - writtenBytes; preMetaDataLength > 0 {
			if err == nil {

				_, err = c.Write(frame[:preMetaDataLength])

				frame = frame[preMetaDataLength:]
				writtenBytes += preMetaDataLength
			}
		}

		if err == nil {

			// Write meta data - no error checking (next write should fail)

			drh.writeStreamMetaData(c, pl)

			// Write rest of the frame

			c.Write(frame)
			writtenBytes += uint64(len(frame))
		}

		writtenBytes -= MetaDataInterval

	} else {

		// Just write the frame to the client

		if err == nil {

			clientWritten, _ := c.Write(frame)

			// Abort if the client does not accept more data

			if clientWritten == 0 && len(frame) > 0 {
				return frameOffset, writtenBytes,
					fmt.Errorf("Could not write to client - closing connection")
			}
		}

		pl.ReleaseFrame(frame)

		writtenBytes += uint64(len(frame))
	}

	return frameOffset, writtenBytes, err
}

/*
writeStreamMetaData writes meta data information into the stream.
*/
func (drh *DefaultRequestHandler) writeStreamMetaData(c net.Conn, playlist Playlist) {
	streamTitle := fmt.Sprintf("StreamTitle='%v - %v';", playlist.Title(), playlist.Artist())

	// Truncate stream title if necessary

	if len(streamTitle) > MaxMetaDataSize {
		streamTitle = streamTitle[:MaxMetaDataSize-2] + "';"
	}

	// Calculate the meta data frame size as a multiple of 16

	metaDataFrameSize := byte(math.Ceil(float64(len(streamTitle)) / 16.0))

	// Write meta data to the client

	metaData := make([]byte, 16.0*metaDataFrameSize+1, 16.0*metaDataFrameSize+1)
	metaData[0] = metaDataFrameSize

	copy(metaData[1:], streamTitle)

	c.Write(metaData)
}

/*
writeStreamStartResponse writes the start response to the client.
*/
func (drh *DefaultRequestHandler) writeStreamStartResponse(c net.Conn,
	name, contentType string, metaDataSupport bool) error {

	c.Write([]byte("ICY 200 OK\r\n"))
	c.Write([]byte(fmt.Sprintf("Content-Type: %v\r\n", contentType)))
	c.Write([]byte(fmt.Sprintf("icy-name: %v\r\n", name)))

	if metaDataSupport {
		c.Write([]byte("icy-metadata: 1\r\n"))
		c.Write([]byte(fmt.Sprintf("icy-metaint: %v\r\n", MetaDataInterval)))
	}

	_, err := c.Write([]byte("\r\n"))

	return err
}

/*
writeStreamNotFoundResponse writes the not found response to the client.
*/
func (drh *DefaultRequestHandler) writeStreamNotFoundResponse(c net.Conn) error {
	_, err := c.Write([]byte("HTTP/1.1 404 Not found\r\n\r\n"))

	return err
}

/*
writeUnauthorized writes the Unauthorized response to the client.
*/
func (drh *DefaultRequestHandler) writeUnauthorized(c net.Conn) error {
	_, err := c.Write([]byte("HTTP/1.1 401 Authorization Required\r\nWWW-Authenticate: Basic realm=\"DudelDu Streaming Server\"\r\n\r\n"))

	return err
}
