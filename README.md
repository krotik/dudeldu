DudelDu
=======
DudelDu is a simple audio streaming server using the SHOUTcast protocol.

<p>
<a href="https://devt.de/build_status.html"><img src="https://devt.de/nightly/build.dudeldu.svg" alt="Build status"></a>
<a href="https://devt.de/nightly/test.dudeldu.html"><img src="https://devt.de/nightly/test.dudeldu.svg" alt="Code coverage"></a>
<a href="https://goreportcard.com/report/github.com/krotik/dudeldu">
<img src="https://goreportcard.com/badge/github.com/krotik/dudeldu?style=flat-square" alt="Go Report Card"></a>
</p>

Features
--------
- Supports various streaming clients: <a href="http://www.videolan.org/vlc/download-windows.en_GB.html">VLC</a>, <a href="https://play.google.com/store/apps/details?id=net.sourceforge.servestream">ServeStream</a>,  ... and most Icecast clients.
- Supports sending of meta data (sending artist and title to the streaming client).
- Playlists are simple json files and data files are normal media (e.g. .mp3) files on disk.
- Can be used as a standalone player or embedded in other Go projects.

Getting Started (standalone application)
----------------------------------------
You can download a precompiled package for Windows (win64) or Linux (amd64) [here](https://devt.de/build_status.html).

The server comes with a demo playlist. Extract the files and go into the directory. You can run the demo playlist with the command:
```
dudeldu.exe demo\demo_playlist.dpl
```
Running the command without any parameters will give you an overview of all available parameters. Point your favourite streaming client or even a browser (Firefox works for me) to the streaming URL:
```
http://localhost:9091/bach/cello_suite1
```
Note: By default you can only reach it via localhost. Use the -host parameter with a host name or IP address to expose it to external network peers.

Building DudelDu
----------------
To build DudelDu from source you need to have Go installed. There a are two options:

### Checkout from github:

Create a directory, change into it and run:
```
git clone https://github.com/krotik/dudeldu/ .
```

Assuming your GOPATH is set to the new directory you should be able to build the binary with:
```
go install devt.de/dudeldu/server
```

### Using go get:

Create a directory, change into it and run:
```
go get -d devt.de/common devt.de/dudeldu
```

Assuming your GOPATH is set to the new directory you should be able to build the binary with:
```
go build devt.de/dudeldu/server
```

License
-------
DudelDu source code is available under the [MIT License](/LICENSE).
