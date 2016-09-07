/*
 * DudelDu
 *
 * Copyright 2016 Matthias Ladkau. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the MIT
 * License, If a copy of the MIT License was not distributed with this
 * file, You can obtain one at https://opensource.org/licenses/MIT.
 */

package playlist

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"devt.de/common/fileutil"
	"devt.de/dudeldu"
)

const pdir = "playlisttest"

const testPlaylist = `
/*
Test comment
*/
{
	"/testpath" : [
		{
			"artist" : "artist1",  // 1234
			"title"  : "test1",
			"path"   : "playlisttest/test1.mp3"
		},
		{
			"artist" : "artist2",
			"title"  : "test2",
			"path"   : "playlisttest/test2.mp4"
		},
		{
			"artist" : "artist3",
			"title"  : "test3",
			"path"   : "playlisttest/test3.mp3"
		}
	]
}`

const testPlaylist2 = `{
	"/testpath" : [
		{
			"artist" : "artist1",
			"title"  : "test1",
			"path"   : "playlisttest/test1.mp3"
		},
		{
			"artist" : "artist2",
			"title"  : "test2",
			"path"   : "playlisttest/test2.mp4"
		},
		{
			"artist" : "artist2",
			"title"  : "test2",
			"path"   : "playlisttest/nonexist"
		},
		{
			"artist" : "artist3",
			"title"  : "test3",
			"path"   : "playlisttest/test3.mp3"
		}
	]
}`

const invalidFileName = "**" + string(0x0)

func TestMain(m *testing.M) {
	flag.Parse()

	// Setup
	if res, _ := fileutil.PathExists(pdir); res {
		os.RemoveAll(pdir)
	}

	err := os.Mkdir(pdir, 0770)
	if err != nil {
		fmt.Print("Could not create test directory:", err.Error())
		os.Exit(1)
	}

	// Run the tests
	res := m.Run()

	// Teardown
	err = os.RemoveAll(pdir)
	if err != nil {
		fmt.Print("Could not remove test directory:", err.Error())
	}

	os.Exit(res)
}

func TestFilePlaylist(t *testing.T) {

	// Set up

	err := ioutil.WriteFile(pdir+"/test1.json", []byte(testPlaylist), 0644)
	if err != nil {
		t.Error(err)
		return
	}

	err = ioutil.WriteFile(pdir+"/test2.json", []byte(testPlaylist2), 0644)
	if err != nil {
		t.Error(err)
		return
	}

	err = ioutil.WriteFile(pdir+"/test1invalid.json", []byte(testPlaylist[2:]), 0644)
	if err != nil {
		t.Error(err)
		return
	}

	err = ioutil.WriteFile(pdir+"/test1.mp3", []byte("123"), 0644)
	if err != nil {
		t.Error(err)
		return
	}
	err = ioutil.WriteFile(pdir+"/test2.mp4", []byte("456789"), 0644)
	if err != nil {
		t.Error(err)
		return
	}
	err = ioutil.WriteFile(pdir+"/test3.mp3", []byte("AB"), 0644)
	if err != nil {
		t.Error(err)
		return
	}

	// Load invalid factory

	_, err = NewFilePlaylistFactory(invalidFileName)
	if err == nil {
		t.Error(err)
		return
	}

	_, err = NewFilePlaylistFactory(pdir + "/test1invalid.json")
	if err.Error() != "invalid character '*' looking for beginning of value" {
		t.Error(err)
		return
	}

	// Create playlist factory

	plf, err := NewFilePlaylistFactory(pdir + "/test1.json")
	if err != nil {
		t.Error(err)
		return
	}

	// Request non-existing path

	res := plf.Playlist("/nonexist", false)

	if res != nil {
		t.Error("Non existing path should return nil")
		return
	}

	// Get existing playlist

	pl := plf.Playlist("/testpath", false)
	defer pl.Close()

	if pl == nil {
		t.Error("Playlist should exist")
		return
	}

	if pl.Name() != "/testpath" {
		t.Error("Unexpected playlist name:", pl.Name())
		return
	}

	FrameSize = 2

	if pl.ContentType() != "audio/mpeg" {
		t.Error("Unexpected content type:", pl.ContentType())
		return
	}

	if pl.Artist() != "artist1" {
		t.Error("Unexpected artist:", pl.ContentType())
		return
	}

	if pl.Title() != "test1" {
		t.Error("Unexpected title:", pl.ContentType())
		return
	}

	// Test close call

	frame, err := pl.Frame()
	if err != nil {
		t.Error(err)
		return
	} else if string(frame) != "12" {
		t.Error("Unexpected frame:", string(frame))
		return
	}

	pl.Close()

	// Make the frame pool run dry if more than one byte array is used

	pl.(*FilePlaylist).framePool = &sync.Pool{}
	pl.(*FilePlaylist).framePool.Put(make([]byte, 2, 2))

	// Check that the right frames are returned

	frame, err = pl.Frame()
	if err != nil {
		t.Error(err)
		return
	} else if string(frame) != "12" {
		t.Error("Unexpected frame:", string(frame))
		return
	}
	pl.ReleaseFrame(frame)

	if pl.Title() != "test1" || pl.Artist() != "artist1" {
		t.Error("Unexpected title/artist:", pl.Title(), pl.Artist())
		return
	}

	frame, err = pl.Frame()
	if err != nil {
		t.Error(err)
		return
	} else if string(frame) != "34" {
		t.Error("Unexpected frame:", string(frame))
		return
	}
	pl.ReleaseFrame(frame)

	if pl.Title() != "test2" || pl.Artist() != "artist2" {
		t.Error("Unexpected title/artist:", pl.Title(), pl.Artist())
		return
	}

	frame, err = pl.Frame()
	if err != nil {
		t.Error(err)
		return
	} else if string(frame) != "56" {
		t.Error("Unexpected frame:", string(frame))
		return
	}
	pl.ReleaseFrame(frame)

	if pl.Title() != "test2" || pl.Artist() != "artist2" {
		t.Error("Unexpected title/artist:", pl.Title(), pl.Artist())
		return
	}

	frame, err = pl.Frame()
	if err != nil {
		t.Error(err)
		return
	} else if string(frame) != "78" {
		t.Error("Unexpected frame:", string(frame))
		return
	}
	pl.ReleaseFrame(frame)

	frame, err = pl.Frame()
	if err != nil {
		t.Error(err)
		return
	} else if string(frame) != "9A" {
		t.Error("Unexpected frame:", string(frame))
		return
	}
	pl.ReleaseFrame(frame)

	if pl.Title() != "test3" || pl.Artist() != "artist3" {
		t.Error("Unexpected title/artist:", pl.Title(), pl.Artist())
		return
	}

	// Check frame pool

	if pl.(*FilePlaylist).framePool.Get() == nil {
		t.Error("Frame pool should have an entry")
		return
	}
	if pl.(*FilePlaylist).framePool.Get() != nil {
		t.Error("Frame pool should have no entry")
		return
	}

	// Put again one byte array back

	pl.(*FilePlaylist).framePool.Put(make([]byte, 2, 2))

	frame, err = pl.Frame()
	if err != dudeldu.ErrPlaylistEnd {
		t.Error(err)
		return
	} else if string(frame) != "B" {
		t.Error("Unexpected frame:", string(frame), frame)
		return
	}
	pl.ReleaseFrame(frame)

	// Check that the byte array was NOT put back into the pool

	if pl.(*FilePlaylist).framePool.Get() != nil {
		t.Error("Frame pool should have no entry")
		return
	}

	if !pl.Finished() {
		t.Error("Playlist should be finished")
		return
	}

	// Change the last file

	err = ioutil.WriteFile(pdir+"/test3.mp3", []byte("A"), 0644)
	if err != nil {
		t.Error(err)
		return
	}

	// Make the frame pool normal again

	pl.(*FilePlaylist).framePool = &sync.Pool{New: func() interface{} { return make([]byte, FrameSize, FrameSize) }}

	// Increase the framesize

	FrameSize = 5
	pl.Close()

	frame, err = pl.Frame()
	if err != nil {
		t.Error(err)
		return
	} else if string(frame) != "12345" {
		t.Error("Unexpected frame:", string(frame), frame)
		return
	}

	// Check that the content type is unknown

	if pl.ContentType() != "audio" {
		t.Error("Content type should be unknown not:", pl.ContentType())
		return
	}

	frame, err = pl.Frame()
	if err != nil {
		t.Error(err)
		return
	} else if string(frame) != "6789A" {
		t.Error("Unexpected frame:", string(frame), frame)
		return
	}

	frame, err = pl.Frame()
	if err != dudeldu.ErrPlaylistEnd {
		t.Error(err)
		return
	} else if string(frame) != "" {
		t.Error("Unexpected frame:", string(frame), frame)
		return
	}

	if !pl.Finished() {
		t.Error("Playlist should be finished")
		return
	}

	// Increase the framesize

	FrameSize = 10
	pl.Close()

	frame, err = pl.Frame()
	if err != nil {
		t.Error(err)
		return
	} else if string(frame) != "123456789A" {
		t.Error("Unexpected frame:", string(frame), frame)
		return
	}

	frame, err = pl.Frame()
	if err != dudeldu.ErrPlaylistEnd {
		t.Error(err)
		return
	} else if string(frame) != "" {
		t.Error("Unexpected frame:", string(frame), frame)
		return
	}

	if !pl.Finished() {
		t.Error("Playlist should be finished")
		return
	}

	// Increase the framesize

	FrameSize = 11
	pl.Close()

	frame, err = pl.Frame()
	if err != dudeldu.ErrPlaylistEnd {
		t.Error(err)
		return
	} else if string(frame) != "123456789A" {
		t.Error("Unexpected frame:", string(frame), frame)
		return
	}

	if !pl.Finished() {
		t.Error("Playlist should be finished")
		return
	}

	// Check that the playlist has finished indeed

	if _, err := pl.Frame(); err != dudeldu.ErrPlaylistEnd {
		t.Error("Playlist end error expected")
		return
	}

	// Create playlist factory

	plf, err = NewFilePlaylistFactory(pdir + "/test2.json")
	if err != nil {
		t.Error(err)
		return
	}

	// Test error

	pl2 := plf.Playlist("/testpath", false)
	defer pl2.Close()

	FrameSize = 6

	frame, err = pl2.Frame()
	if err != nil {
		t.Error(err)
		return
	} else if string(frame) != "123456" {
		t.Error("Unexpected frame:", string(frame), frame)
		return
	}

	frame, err = pl2.Frame()
	if err.Error() != "open playlisttest/nonexist: The system cannot find the file specified." &&
		err.Error() != "open playlisttest/nonexist: no such file or directory" {
		t.Error(err)
		return
	} else if string(frame) != "789" {
		t.Error("Unexpected frame:", string(frame), frame)
		return
	}

	frame, err = pl2.Frame()
	if err != dudeldu.ErrPlaylistEnd {
		t.Error(err)
		return
	} else if string(frame) != "A" {
		t.Error("Unexpected frame:", string(frame), frame)
		return
	}

	// Make sure currentItem does not blow up

	if pl2.Title() != "test3" {
		t.Error("Unexpected result:", pl2.Title())
		return
	}

	// Test shuffling

	pl3 := plf.Playlist("/testpath", true)

	if len(pl3.(*FilePlaylist).data) != len(pl2.(*FilePlaylist).data) {
		t.Error("Length of playlists differ")
		return
	}
}
