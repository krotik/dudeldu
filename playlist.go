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

import "errors"

/*
FrameSize is the suggested size of a frame which should be send to the client
at a time.

The absolute theoretical maximum frame size for a MPEG audio is 2881 bytes:

MPEG 2.5 Layer II, 8000 Hz @ 160 kbps, with a padding slot.
Theoretical frame sizes for Layer III range from 24 to 1441 bytes
there is a "soft" limit imposed by the standard of 960 bytes.

see: http://www.mars.org/pipermail/mad-dev/2002-January/000425.html
*/
const FrameSize = 3000

/*
ErrPlaylistEnd is a special error code which signals that the end of the playlist has been reached
*/
var ErrPlaylistEnd = errors.New("End of playlist")

/*
Playlist is an object which provides a request handler with a
constant stream of bytes and meta information about the current playing title.
*/
type Playlist interface {

	/*
	   Name is the name of the playlist.
	*/
	Name() string

	/*
	   ContentType returns the content type of this playlist e.g. audio/mpeg.
	*/
	ContentType() string

	/*
	   Artist returns the artist which is currently playing.
	*/
	Artist() string

	/*
	   Title returns the title which is currently playing.
	*/
	Title() string

	/*
		Frame returns the current audio frame which is playing.
	*/
	Frame() ([]byte, error)

	/*
		ReleaseFrame releases a frame which has been written to the client.
	*/
	ReleaseFrame([]byte)

	/*
		Finished returns if the playlist has finished playing.
	*/
	Finished() bool

	/*
		Close any open files by this playlist and reset the current pointer. After this
		call the playlist can be played again unless an error is returned.
	*/
	Close() error
}

/*
PlaylistFactory produces a Playlist for a given path.
*/
type PlaylistFactory interface {

	/*
		Playlist returns a playlist for a given path.
	*/
	Playlist(path string, shuffle bool) Playlist
}
