# Building the Vogo Client

## MacOS

1. [Install Go](https://go.dev/doc/install)

2. Install opus with [homebrew](https://brew.sh/)
`brew install opus`

##### Installing for normal use:
`make install`

##### Development usage:
- Build and run: `make run`
- Test: `make test`

## Windows

> For building and testing locally. 

1. Install [mingw64](https://winlibs.com/) (UCRT)
This could be done via [MSYS2](https://www.msys2.org/), but I prefer to install [git for windows](https://git-scm.com/install/windows), which gives you "Git Bash", which runs a mingw64 shell. The following steps should all be done inside a mingw64 shell. 

2. Install Go
I used the official windows installer, then added Go dirs to Git Bash's PATH

3. Download libopus
- `curl -L https://downloads.xiph.org/releases/opus/opus-1.6.1.tar.gz -o opus.tar.gz`
- place into a dir of your choosing: `tar -xzf opus.tar.gz`
- create a dir for go to find C dependencies: `mkdir /c/deps`

4. Build libopus (from the parent dir of where you placed opus)
`cmake -S opus-1.6.1 -B opus-1.6.1/build -G "MinGW Makefiles" -DOPUS_BUILD_SHARED_LIBRARY=ON -DOPUS_X86_PRESUME_AVX2=ON -DOPUS_X86_PRESUME_SSE2=ON -DOPUS_X86_PRESUME_SSE4_1=ON -DOPUS_DEEP_PLC=ON -DOPUS_OSCE=ON -DCMAKE_C_FLAGS="-march=x86-64-v3 -O3" -DCMAKE_INSTALL_PREFIX=/c/deps/opus
cmake --build opus-1.6.1/build -parallel
cmake --install opus-1.6.1/build`
- Move DLL to the same dir where you will put the vogo binary: `cp /c/deps/opus/bin/libopus.dll [path-to-vogo-cli-repo]/bin/libopus.dll`

5. Build a binary
- cd into vogo cli dir
`GOEXPERIMENT=simd CC=gcc CGO_CFLAGS=-I/c/deps/opus/include/opus CGO_LDFLAGS="-L/c/deps/opus/lib -lopus" PKG_CONFIG=true go build -tags nolibopusfile -o bin/vogo.exe main.go`
> Now you can run the vogo binary: `./bin/vogo.exe`

6. Build a test binary to benchmark audio mixing: 
`GOEXPERIMENT=simd CC=gcc CGO_CFLAGS=-I/c/deps/opus/include/opus CGO_LDFLAGS="-L/c/deps/opus/lib -lopus" PKG_CONFIG=true go test -c -o ../bin/vogotest.exe -v -bench=. -benchmem -cpu=1 -tags nolibopusfile -v ./internal/audio`
- run the tests: `./bin/vogotest.exe -test.bench=. -test.benchmem -test.cpu=1`
