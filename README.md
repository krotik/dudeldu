DudelDu
=======
DudelDu is a simple audio/video streaming server using the SHOUTcast protocol.

<p>
<a href="https://void.devt.de/pub/dudeldu/coverage.txt"><img src="https://void.devt.de/pub/dudeldu/test_result.svg" alt="Code coverage"></a>
<a href="https://goreportcard.com/report/devt.de/krotik/dudeldu">
<img src="https://goreportcard.com/badge/devt.de/krotik/dudeldu?style=flat-square" alt="Go Report Card"></a>
<a href="https://godoc.org/devt.de/krotik/dudeldu">
<img src="https://godoc.org/devt.de/krotik/dudeldu?status.svg" alt="Go Doc"></a>
</p>

Features
--------
- Supports various streaming clients: <a href="http://www.videolan.org/vlc/download-windows.en_GB.html">VLC</a>, <a href="https://play.google.com/store/apps/details?id=net.sourceforge.servestream">ServeStream</a>,  ... and most Icecast clients.
- Supports sending of meta data (sending artist and title to the streaming client).
- Playlists are simple JSON files and data files are normal media (e.g. `.mp3`, `.nsv`) files on disk.
- Can be used as a stand-alone server or embedded in other Go projects.
- Supports HTTP basic user authentication.

Getting Started (standalone application)
----------------------------------------
You can download a pre-compiled package for Windows (win64) or Linux (amd64) [here](https://void.devt.de/pub/dudeldu).

You can also pull the latest docker image of DudelDu from [Dockerhub](https://hub.docker.com/r/krotik/dudeldu):
```
docker pull krotik/dudeldu
```

Create an empty directory, change into it and run the following to start DudelDu:
```
docker run --rm --user $(id -u):$(id -g) -v $PWD:/data -p 9091:9091 krotik/dudeldu -host 0.0.0.0 <playlist>
```
The container will have access to the current local directory and all subfolders.

### Demo

DudelDu comes with a demo playlist. After extracting DudelDu switch to the directory `examples/demo`. Run ./run_demo.sh (Linux) or run_demo.bat (Windows) to start the server.

Open a browser and view the `demo.html` in the `examples/demo` directory. To access the demo streams you are prompted for a username and password. The credentials are:
```
username: web
password: web
```
You can also point your favourite audio streaming client (e.g. VLC) to the streaming URL:
```
http://localhost:9091/bach/cello_suite1
```
The demo includes also a small video in the [Nullsoft Streaming Video](https://en.wikipedia.org/wiki/Nullsoft_Streaming_Video) format (NSV). To see it point a video streaming client (e.g. VLC) to:
```
http://localhost:9091/trailer/big_buck_bunny
```
Note: By default you can only reach the streams via localhost. Use the -host parameter with a host name or IP address to expose it to external network peers.

### Command line options
The main DudelDu executable has the following command line options:
```
DudelDu x.x.x
Usage of ./dudeldu [options] <playlist>
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

Authentication can also be defined via the environment variable: DUDELDU_AUTH="<user>:<pass>"
```

Building DudelDu
----------------
To build DudelDu from source you need to have Go installed (go >= 1.12):

Create a directory, change into it and run:
```
git clone https://devt.de/krotik/dudeldu/ .
```

You can build DudelDu's executable with:
```
go build ./server/dudeldu.go
```

Building DudelDu as Docker image
--------------------------------
DudelDu can be build as a secure and compact Docker image.

- Create a directory, change into it and run:
```
git clone https://devt.de/krotik/dudeldu/ .
```

- You can now build the Docker image with:
```
docker build --tag krotik/dudeldu .
```

License
-------
DudelDu source code is available under the [MIT License](/LICENSE).
