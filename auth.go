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
Package dudeldu is a simple audio streaming server using the SHOUTcast protocol.

Server

Server is the main server object which runs a shoutcast server instance.

Using a WaitGroup a client can wait for the start and shutdown of the server.
Incoming new connections are served with a ConnectionHandler method. The
default implementation for this is the HandleRequest method of the
DefaultRequestHandler object.

DefaultRequestHandler

DefaultRequestHandler is the default request handler implementation for the
DudelDu server. DefaultRequestHandler has a customizable ServeRequest function.
ServeRequest is called once a request was successfully decoded.

The default implementation supports sending meta data while streaming audio. The
metadata implementation is according to:

http://www.smackfu.com/stuff/programming/shoutcast.html

Playlists

Playlists provide the data which is send to the client. A simple implementation
will just read .mp3 files and send them in chunks (via the Frame() method) to
the client.

A request handler uses a PlaylistFactory to produce a Playlist for each new
connection.
*/
package dudeldu

import (
	"encoding/base64"
	"regexp"
)

/*
requestAuthPattern is the pattern which is used to extract the request authentication
(i case-insensitive / m multi-line mode: ^ and $ match begin/end line)
*/
var requestAuthPattern = regexp.MustCompile("(?im)^Authorization: Basic (\\S+).*$")

/*
checkAuth checks the authentication header of a client request.
*/
func (drh *DefaultRequestHandler) checkAuth(bufStr string, clientString string) (string, string, bool) {

	auth := ""
	res := requestAuthPattern.FindStringSubmatch(bufStr)
	origBufStr, hasAuth := drh.authPeers.Get(clientString)

	if len(res) > 1 {

		// Decode authentication

		b, err := base64.StdEncoding.DecodeString(res[1])
		if err != nil {
			drh.logger.PrintDebug("Invalid request (cannot decode authentication): ", bufStr)
			return auth, bufStr, false
		}

		auth = string(b)

		// Authorize request

		if auth != drh.auth && drh.auth != "" {
			drh.logger.PrintDebug("Wrong authentication:", auth)
			return auth, bufStr, false
		}

		// Peer is now authorized store this so it can connect again

		drh.authPeers.Put(clientString, bufStr)

	} else if drh.auth != "" && !hasAuth {

		// No authorization

		drh.logger.PrintDebug("No authentication found")
		return auth, bufStr, false

	} else if bufStr == "" && hasAuth {

		// Workaround for strange clients like VLC which send first the
		// authentication then connect again on a different port and just
		// expect the stream

		bufStr = origBufStr.(string)

		// Get again the authentication

		res = requestAuthPattern.FindStringSubmatch(bufStr)

		if len(res) > 1 {
			if b, err := base64.StdEncoding.DecodeString(res[1]); err == nil {
				auth = string(b)
			}
		}
	}

	return auth, bufStr, true
}
