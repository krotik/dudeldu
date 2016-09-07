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
Package dudeldu contains a shoutcast server implementation.

DefaultRequestHandler is the default request handler implementation for the
DudelDu server. DefaultRequestHandler has a customizable ServeRequest function.
ServeRequest is called once a request was successfully decoded.

The default implementation supports sending meta data while streaming audio. The
metadata implementation is according to:

http://www.smackfu.com/stuff/programming/shoutcast.html
*/
package dudeldu

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
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
Logger is a function which receives log messages
*/
type Logger func(v ...interface{})

/*
Print logger method. Using a custom type so it can be customized.
*/
var Print Logger = log.Print

/*
DebugOutput is a flag to enable additional debugging output
*/
var DebugOutput = false

/*
DefaultRequestHandler data structure
*/
type DefaultRequestHandler struct {
	PlaylistFactory PlaylistFactory // Factory for playlists
	ServeRequest    func(c net.Conn, path string,
		metaDataSupport bool, offset int) // Function to serve requests
	loop      bool // Flag if the playlist should be looped
	LoopTimes int  // Number of loops -1 loops forever
	shuffle   bool // Flag if the playlist should be shuffled
}

/*
NewDefaultRequestHandler creates a new default request handler object.
*/
func NewDefaultRequestHandler(pf PlaylistFactory, loop bool, shuffle bool) *DefaultRequestHandler {
	drh := &DefaultRequestHandler{
		PlaylistFactory: pf,
		loop:            loop,
		LoopTimes:       -1,
		shuffle:         shuffle,
	}
	drh.ServeRequest = drh.defaultServeRequest
	return drh
}

/*
HandleRequest handles requests from streaming clients. It tries to extract
the path and if meta data is supported. Once a request has been successfully
decoded ServeRequest is called. The connection is closed once HandleRequest
finishes.
*/
func (drh *DefaultRequestHandler) HandleRequest(c net.Conn, nerr net.Error) {

	if DebugOutput {
		Print("Handling request from: ", c.RemoteAddr())
	}

	defer func() {
		c.Close()
	}()

	// Check if there was an error

	if nerr != nil {
		Print(nerr)
		return
	}

	rbuf := make([]byte, 512, 512)
	var buf bytes.Buffer

	// Decode request

	n, err := c.Read(rbuf)

	for n > 0 || (err != nil && err != io.EOF) {

		// Do some error checking

		if err != nil {
			Print(err)
			return
		} else if buf.Len() > MaxRequestSize {
			Print("Illegal request: Request is too long")
			return
		}

		buf.Write(rbuf[:n])

		if strings.Contains(string(rbuf), "\r\n\r\n") {
			break
		}

		n, err = c.Read(rbuf)
	}

	// Add ending sequence in case the client "forgets"

	bufStr := buf.String() + "\r\n\r\n"

	if DebugOutput {
		Print("Request:", bufStr)
	}

	if i := strings.Index(bufStr, "\r\n\r\n"); i >= 0 {
		bufStr = strings.TrimSpace(bufStr[:i])

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

			drh.ServeRequest(c, res[1], metaDataSupport, offset)

			return
		}
	}

	Print("Invalid request: ", bufStr)
}

/*
defaultServeRequest is called once a request was successfully decoded.
*/
func (drh *DefaultRequestHandler) defaultServeRequest(c net.Conn, path string, metaDataSupport bool, offset int) {
	var err error

	if DebugOutput {
		Print("Serve request path:", path, " Metadata support:", metaDataSupport, " Offset:", offset)
	}

	pl := drh.PlaylistFactory.Playlist(path, drh.shuffle)
	if pl == nil {

		// Stream was not found - no error checking here (don't care)

		drh.writeStreamNotFoundResponse(c)
		return
	}

	err = drh.writeStreamStartResponse(c, pl.Name(), pl.ContentType(), metaDataSupport)

	clientWritten := 0
	var writtenBytes uint64
	currentPlaying := ""
	frameOffset := offset

	for {

		for !pl.Finished() {

			if DebugOutput {
				playingString := fmt.Sprintf("%v - %v", pl.Title(), pl.Artist())

				if playingString != currentPlaying {
					currentPlaying = playingString
					Print("Written bytes: ", writtenBytes)
					Print("Sending: ", currentPlaying)
				}
			}

			// Check if there were any errors

			if err != nil {
				Print(err)
				return
			}

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
					Print(fmt.Sprintf("Empty frame for: %v - %v (Error: %v)", pl.Title(), pl.Artist(), err))
				}
				continue
			} else if err != nil {
				if err != ErrPlaylistEnd {
					Print(fmt.Sprintf("Error while retrieving playlist data: %v", err))
				}
				err = nil
			}

			// Check if meta data should be send

			if metaDataSupport && writtenBytes+uint64(len(frame)) >= MetaDataInterval {

				// Write rest data before sending meta data

				preMetaDataLength := MetaDataInterval - writtenBytes
				if preMetaDataLength > 0 {
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

					_, err = c.Write(frame)
					writtenBytes += uint64(len(frame))
				}

				writtenBytes -= MetaDataInterval

			} else {

				// Just write the frame to the client

				if err == nil {

					clientWritten, err = c.Write(frame)

					// Abort if the client does not accept more data

					if clientWritten == 0 && len(frame) > 0 {
						Print(fmt.Sprintf("Could not write to client - closing connection"))
						return
					}
				}

				pl.ReleaseFrame(frame)

				writtenBytes += uint64(len(frame))
			}
		}

		pl.Close()

		// Handle looping

		if !drh.loop {
			break
		} else if drh.LoopTimes != -1 {
			drh.LoopTimes--
			if drh.LoopTimes == 0 {
				break
			}
		}
	}

	if DebugOutput {
		Print("Serve request path:", path, " complete")
	}
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
	_, err := c.Write([]byte("HTTP/1.0 404 Not found\r\n\r\n"))

	return err
}
