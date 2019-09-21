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
Package playlist contains the default playlist implementation.

FilePlaylistFactory

FilePlaylistFactory is a PlaylistFactory which reads its definition from
a file. The definition file is expected to be a JSON encoded datastructure of the form:

	{
	    <web path> : [
	        {
	            "artist" : <artist>
	            "title"  : <title>
	            "path"   : <file path / url>
	        }
	    ]
	}

The web path is the absolute path which may be requested by the streaming
client (e.g. /foo/bar would be http://myserver:1234/foo/bar).
The path is either a physical file or a web url reachable by the server process.
The file ending determines the content type which is send to the client.
*/
package playlist

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"devt.de/krotik/common/stringutil"
	"devt.de/krotik/dudeldu"
)

/*
FileExtContentTypes maps file extensions to content types
*/
var FileExtContentTypes = map[string]string{
	".mp3":  "audio/mpeg",
	".flac": "audio/flac",
	".aac":  "audio/x-aac",
	".mp4a": "audio/mp4",
	".mp4":  "video/mp4",
	".nsv":  "video/nsv",
	".ogg":  "audio/ogg",
	".spx":  "audio/ogg",
	".opus": "audio/ogg",
	".oga":  "audio/ogg",
	".ogv":  "video/ogg",
	".weba": "audio/webm",
	".webm": "video/webm",
	".axa":  "audio/annodex",
	".axv":  "video/annodex",
}

/*
FrameSize is the frame size which is used by the playlists
*/
var FrameSize = dudeldu.FrameSize

/*
FilePlaylistFactory data structure
*/
type FilePlaylistFactory struct {
	data           map[string][]map[string]string
	itemPathPrefix string
}

/*
NewFilePlaylistFactory creates a new FilePlaylistFactory from a given definition
file.
*/
func NewFilePlaylistFactory(path string, itemPathPrefix string) (*FilePlaylistFactory, error) {

	// Try to read the playlist file

	pl, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Unmarshal json

	ret := &FilePlaylistFactory{
		data:           nil,
		itemPathPrefix: itemPathPrefix,
	}

	err = json.Unmarshal(pl, &ret.data)

	if err != nil {

		// Try again and strip out comments

		pl = stringutil.StripCStyleComments(pl)

		err = json.Unmarshal(pl, &ret.data)
	}

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

		return &FilePlaylist{path, fp.itemPathPrefix, 0, data, nil, false,
			&sync.Pool{New: func() interface{} { return make([]byte, FrameSize, FrameSize) }}}
	}
	return nil
}

/*
FilePlaylist data structure
*/
type FilePlaylist struct {
	path       string              // Path of this playlist
	pathPrefix string              // Prefix for all paths
	current    int                 // Pointer to the current playing item
	data       []map[string]string // Playlist items
	stream     io.ReadCloser       // Current open stream
	finished   bool                // Flag if this playlist has finished
	framePool  *sync.Pool          // Pool for byte arrays
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
	ext := filepath.Ext(fp.currentItem()["path"])

	if ctype, ok := FileExtContentTypes[ext]; ok {
		return ctype
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

	if fp.stream == nil {

		// Make sure first file is loaded

		err = fp.nextFile()
	}

	if err == nil {

		// Get new byte array from a pool

		frame = fp.framePool.Get().([]byte)

		n := 0
		nn := 0

		for n < len(frame) && err == nil {

			nn, err = fp.stream.Read(frame[n:])
			n += nn

			// Check if we need to read the next file

			if n < len(frame) || err == io.EOF {
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
	var err error
	var stream io.ReadCloser

	// Except for the first call advance the current pointer

	if fp.stream != nil {
		fp.current++

		fp.stream.Close()
		fp.stream = nil

		// Return special error if the end of the playlist has been reached

		if fp.current >= len(fp.data) {
			return dudeldu.ErrPlaylistEnd
		}
	}

	// Check if a file is already open

	if fp.stream == nil {

		item := fp.pathPrefix + fp.currentItem()["path"]

		if _, err = url.ParseRequestURI(item); err == nil {
			var resp *http.Response

			// We got an url - access it without SSL verification

			client := &http.Client{Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}}

			if resp, err = client.Get(item); err == nil {
				buf := &StreamBuffer{}
				buf.ReadFrom(resp.Body)
				stream = buf
			}

		} else {

			// Open a new file

			stream, err = os.Open(item)
		}

		if err != nil {

			// Jump to the next file if there is an error

			fp.current++

			return err
		}

		fp.stream = stream
	}

	return err
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
func (fp *FilePlaylist) Close() error {
	if fp.stream != nil {
		fp.stream.Close()
		fp.stream = nil
	}
	fp.current = 0
	fp.finished = false

	return nil
}

/*
StreamBuffer is a buffer which implements io.ReadCloser and can be used to stream
one stream into another. The buffer detects a potential underflow and waits
until enough bytes were read from the source stream.
*/
type StreamBuffer struct {
	bytes.Buffer    // Buffer which is used to hold the data
	readFromOngoing bool
}

func (b *StreamBuffer) Read(p []byte) (int, error) {

	if b.readFromOngoing && b.Buffer.Len() < len(p) {

		// Prevent buffer underflow and wait until we got enough data for
		// the next read

		time.Sleep(10 * time.Millisecond)
		return b.Read(p)
	}

	n, err := b.Buffer.Read(p)

	// Return EOF if the buffer is empty

	if err == nil {
		if _, err = b.ReadByte(); err == nil {
			b.UnreadByte()
		}
	}

	return n, err
}

/*
ReadFrom reads the source stream into the buffer.
*/
func (b *StreamBuffer) ReadFrom(r io.Reader) (int64, error) {
	b.readFromOngoing = true
	go func() {
		b.Buffer.ReadFrom(r)
		b.readFromOngoing = false
	}()
	return 0, nil
}

/*
Close does nothing but must be there to implement io.ReadCloser.
*/
func (b *StreamBuffer) Close() error {

	// We are in memory so no need to close anything

	return nil
}
