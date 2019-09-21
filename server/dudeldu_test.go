/*
 * DudelDu
 *
 * Copyright 2016 Matthias Ladkau. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the MIT
 * License, If a copy of the MIT License was not distributed with this
 * file, You can obtain one at https://opensource.org/licenses/MIT.
 */

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"devt.de/krotik/common/fileutil"
	"devt.de/krotik/common/testutil"
	"devt.de/krotik/dudeldu"
	"devt.de/krotik/dudeldu/playlist"
)

const testFilePlaylist = `
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

const pdir = "playlisttest"

func TestRequestHandlerFilePlaylist(t *testing.T) {

	var out bytes.Buffer

	// Collect the print output
	dudeldu.Print = func(v ...interface{}) {
		out.WriteString(fmt.Sprint(v...))
		out.WriteString("\n")
	}
	defer func() {
		dudeldu.Print = log.Print
	}()

	os.Mkdir(pdir, 0770)
	defer func() {
		os.RemoveAll(pdir)
	}()

	ioutil.WriteFile(pdir+"/test.dpl", []byte(testFilePlaylist), 0644)
	ioutil.WriteFile(pdir+"/test1.mp3", []byte("abcdefgh"), 0644)
	ioutil.WriteFile(pdir+"/test2.mp4", []byte("12345"), 0644)
	ioutil.WriteFile(pdir+"/test3.mp3", []byte("???!!!&&&$$$"), 0644)

	fac, err := playlist.NewFilePlaylistFactory(pdir+"/test.dpl", "")
	if err != nil {
		t.Error(err)
		return
	}

	drh := dudeldu.NewDefaultRequestHandler(fac, false, false, "")
	testConn := &testutil.ErrorTestingConnection{}
	dudeldu.MetaDataInterval = 5
	playlist.FrameSize = 5

	drh.ServeRequest(testConn, "/testpath", true, 2, "")

	if testConn.Out.String() != ("ICY 200 OK\r\n" +
		"Content-Type: audio/mpeg\r\n" +
		"icy-name: /testpath\r\n" +
		"icy-metadata: 1\r\n" +
		"icy-metaint: 5\r\n" +
		"\r\n" +
		`cdefg` + string(0x02) + `StreamTitle='test2 - artist2';` + string([]byte{0x0, 0x0}) +
		`h1234` + string(0x02) + `StreamTitle='test3 - artist3';` + string([]byte{0x0, 0x0}) +
		`5???!` + string(0x02) + `StreamTitle='test3 - artist3';` + string([]byte{0x0, 0x0}) +
		`!!&&&` + string(0x02) + `StreamTitle='test3 - artist3';` + string([]byte{0x0, 0x0}) +
		`$$$`) {

		t.Error("Unexpected response:", testConn.Out.String())
		return
	}

}

func TestDudelDuMain(t *testing.T) {

	// Make the fatal a simple print

	fatal = print

	// Make sure out.txt and test.dpl are removed

	defer func() {
		if res, _ := fileutil.PathExists("out.txt"); res {
			os.Remove("out.txt")
		}
		if res, _ := fileutil.PathExists("test.dpl"); res {
			os.Remove("test.dpl")
		}
	}()

	// Reset flags

	flag.CommandLine = &flag.FlagSet{}

	// Test usage text

	os.Args = []string{"dudeldu", "-?", "-port", "9000", "test"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	if ret, err := execMain(); err != nil || ret != `
DudelDu `[1:]+dudeldu.ProductVersion+`
Usage of dudeldu [options] <playlist>
  -?	Show this help message
  -auth string
    	Authentication as <user>:<pass>
  -debug
    	Enable extra debugging output
  -fqs int
    	Frame queue size (default 10000)
  -host string
    	Server hostname to listen on (default "127.0.0.1")
  -loop
    	Loop playlists
  -port string
    	Server port to listen on (default "9091")
  -pp string
    	Prefix all paths with a string
  -shuffle
    	Shuffle playlists
  -tps int
    	Thread pool size (default 10)
` {
		t.Error("Unexpected output:", "#"+ret+"#", err)
		return
	}

	ioutil.WriteFile("test.dpl", []byte("{}"), 0644)

	os.Args = []string{"dudeldu", "-auth", "web:web", "-port", "-1", "test.dpl"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	if ret, err := execMain(); err != nil || ret != `
DudelDu `[1:]+dudeldu.ProductVersion+`
Serving playlist test.dpl on 127.0.0.1:-1
Thread pool size: 10
Frame queue size: 10000
Loop playlist: false
Shuffle playlist: false
Path prefix: 
Required authentication: web:web
listen tcp: invalid port -1
Shutting down
` && ret != `
DudelDu `[1:]+dudeldu.ProductVersion+`
Serving playlist test.dpl on 127.0.0.1:-1
Thread pool size: 10
Frame queue size: 10000
Loop playlist: false
Shuffle playlist: false
Path prefix: 
Required authentication: web:web
listen tcp: address -1: invalid port
Shutting down
` {
		t.Error("Unexpected output:", ret, err)
		return
	}
}

/*
Execute the main function and capture the output.
*/
func execMain() (string, error) {

	// Exchange stderr to a file

	origStdErr := os.Stderr
	outFile, err := os.Create("out.txt")
	if err != nil {
		return "", err
	}
	defer func() {
		outFile.Close()
		os.RemoveAll("out.txt")

		// Put Stderr back

		os.Stderr = origStdErr
	}()

	os.Stderr = outFile

	main()

	outFile.Sync()

	out, err := ioutil.ReadFile("out.txt")
	if err != nil {
		return "", err
	}

	return string(out), nil
}
