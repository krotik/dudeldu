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
Package playlist contains default playlist implementations.

FilePlaylistFactory is a PlaylistFactory which reads its definition from
a file.
*/
package playlist

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"devt.de/common/stringutil"
	"devt.de/dudeldu"
)

/*
FrameSize is the frame size which is used by the playlists
*/
var FrameSize = dudeldu.FrameSize

/*
FilePlaylistFactory data structure
*/
type FilePlaylistFactory struct {
	data map[string][]map[string]string
}

/*
NewFilePlaylistFactory creates a new FilePlaylistFactory from a given definition
file. The definition file is expected to be a JSON encoded datastructure of the form:

{
	<web path> : [
		{
			"artist" : <artist>
			"title"  : <title>
			"path"   : <file path>
		}
	]
}

The web path is the absolute path which may be requested by the streaming
client (e.g. /foo/bar would be http://myserver:1234/foo/bar).
The file path is a physical file reachable by the server process. The file
ending determines the content type which is send to the client.
*/
func NewFilePlaylistFactory(path string) (*FilePlaylistFactory, error) {

	// Try to read the playlist file

	pl, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Strip out comments

	pl = stringutil.StripCStyleComments(pl)

	// Unmarshal json

	ret := &FilePlaylistFactory{}

	err = json.Unmarshal(pl, &ret.data)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

/*
Playlist returns a playlist for a given path.
*/
func (fp *FilePlaylistFactory) Playlist(path string, shuffle bool) dudeldu.Playlist {
	if data, ok := fp.data[path]; ok {

		// Check if the playlist should be shuffled

		if shuffle {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))

			shuffledData := make([]map[string]string, len(data), len(data))

			for i, j := range r.Perm(len(data)) {
				shuffledData[i] = data[j]
			}

			data = shuffledData
		}

		return &FilePlaylist{path, 0, data, nil, false,
			&sync.Pool{New: func() interface{} { return make([]byte, FrameSize, FrameSize) }}}
	}
	return nil
}

/*
FilePlaylist data structure
*/
type FilePlaylist struct {
	path      string              // Path of this playlist
	current   int                 // Pointer to the current playing item
	data      []map[string]string // Playlist items
	file      *os.File            // Current open file
	finished  bool                // Flag if this playlist has finished
	framePool *sync.Pool          // Pool for byte arrays
}

/*
currentItem returns the current playlist item
*/
func (fp *FilePlaylist) currentItem() map[string]string {
	if fp.current < len(fp.data) {
		return fp.data[fp.current]
	}

	return fp.data[len(fp.data)-1]
}

/*
Name is the name of the playlist.
*/
func (fp *FilePlaylist) Name() string {
	return fp.path
}

/*
ContentType returns the content type of this playlist e.g. audio/mpeg.
*/
func (fp *FilePlaylist) ContentType() string {
	c := fp.currentItem()

	if strings.HasSuffix(c["path"], ".mp3") {
		return "audio/mpeg"
	}

	return "audio"
}

/*
Artist returns the artist which is currently playing.
*/
func (fp *FilePlaylist) Artist() string {
	return fp.currentItem()["artist"]
}

/*
Title returns the title which is currently playing.
*/
func (fp *FilePlaylist) Title() string {
	return fp.currentItem()["title"]
}

/*
Frame returns the current audio frame which is playing.
*/
func (fp *FilePlaylist) Frame() ([]byte, error) {
	var err error
	var frame []byte

	if fp.finished {
		return nil, dudeldu.ErrPlaylistEnd
	}

	if fp.file == nil {

		// Make sure first file is loaded

		err = fp.nextFile()
	}

	if err == nil {

		// Get new byte array from a pool

		frame = fp.framePool.Get().([]byte)

		n := 0
		nn := 0

		for n < len(frame) && err == nil {

			nn, err = fp.file.Read(frame[n:])

			n += nn

			// Check if we need to read the next file

			if n < len(frame) {
				err = fp.nextFile()
			}
		}

		// Make sure the frame has no old data if it was only partially filled

		if n == 0 {

			// Special case we reached the end of the playlist

			frame = nil
			if err != nil {
				err = dudeldu.ErrPlaylistEnd
			}

		} else if n < len(frame) {

			// Resize frame if we have less data

			frame = frame[:n]
		}
	}

	if err == dudeldu.ErrPlaylistEnd {
		fp.finished = true
	}

	return frame, err
}

/*
nextFile jumps to the next file for the playlist.
*/
func (fp *FilePlaylist) nextFile() error {

	// Except for the first call advance the current pointer

	if fp.file != nil {
		fp.current++

		fp.file.Close()
		fp.file = nil

		// Return special error if the end of the playlist has been reached

		if fp.current >= len(fp.data) {
			return dudeldu.ErrPlaylistEnd
		}
	}

	// Check if a file is already open

	if fp.file == nil {

		// Open a new file

		f, err := os.Open(fp.currentItem()["path"])
		if err != nil {

			// Jump to the next file if there is an error

			fp.current++

			return err
		}

		fp.file = f
	}

	return nil
}

/*
ReleaseFrame releases a frame which has been written to the client.
*/
func (fp *FilePlaylist) ReleaseFrame(frame []byte) {
	if len(frame) == FrameSize {
		fp.framePool.Put(frame)
	}
}

/*
Finished returns if the playlist has finished playing.
*/
func (fp *FilePlaylist) Finished() bool {
	return fp.finished
}

/*
Close any open files by this playlist and reset the current pointer. After this
call the playlist can be played again.
*/
func (fp *FilePlaylist) Close() {
	if fp.file != nil {
		fp.file.Close()
		fp.file = nil
	}
	fp.current = 0
	fp.finished = false
}
