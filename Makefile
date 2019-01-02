NAME=clash
BINDIR=bin
GOBUILD=CGO_ENABLED=0 go build -ldflags '-w -s'

PLATFORM_LIST = \
	darwin-amd64 \
	linux-386 \
	linux-amd64 \
	linux-armv5 \
	linux-armv6 \
	linux-armv7 \
	linux-armv8 \
	linux-mips-softfloat \
	linux-mips-hardfloat \
	linux-mipsle \
	linux-mips64 \
	linux-mips64le \
	freebsd-386 \
	freebsd-amd64

WINDOWS_ARCH_LIST = \
	windows-386 \
	windows-amd64

all: linux-amd64 darwin-amd64 windows-amd64 # Most used

darwin-amd64:
	GOARCH=amd64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-386:
	GOARCH=386 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-amd64:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv5:
	GOARCH=arm GOOS=linux GOARM=5 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv6:
	GOARCH=arm GOOS=linux GOARM=6 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv7:
	GOARCH=arm GOOS=linux GOARM=7 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv8:
	GOARCH=arm64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips-softfloat:
	GOARCH=mips GOMIPS=softfloat GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips-hardfloat:
	GOARCH=mips GOMIPS=hardfloat GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mipsle:
	GOARCH=mipsle GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips64:
	GOARCH=mips64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips64le:
	GOARCH=mips64le GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

freebsd-386:
	GOARCH=386 GOOS=freebsd $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

freebsd-amd64:
	GOARCH=amd64 GOOS=freebsd $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

windows-386:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

windows-amd64:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

gz_releases=$(addsuffix .gz, $(PLATFORM_LIST))
zip_releases=$(addsuffix .zip, $(WINDOWS_ARCH_LIST))

$(gz_releases): %.gz : %
	chmod +x $(BINDIR)/$(NAME)-$(basename $@)
	gzip -f $(BINDIR)/$(NAME)-$(basename $@)

$(zip_releases): %.zip : %
	zip -m -j $(BINDIR)/$(NAME)-$(basename $@).zip $(BINDIR)/$(NAME)-$(basename $@).exe

all-arch: $(PLATFORM_LIST) $(WINDOWS_ARCH_LIST)

releases: $(gz_releases) $(zip_releases)
clean:
	rm $(BINDIR)/*
