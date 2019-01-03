NAME=clash
BINDIR=bin
GOBUILD=CGO_ENABLED=0 go build -ldflags '-w -s'

all: linux macos win64 arm

linux:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

macos:
	GOARCH=amd64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

win64:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

arm:
	GOARM=5 GOARCH=arm GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

releases: linux macos win64
	chmod +x $(BINDIR)/$(NAME)-*
	gzip $(BINDIR)/$(NAME)-linux
	gzip $(BINDIR)/$(NAME)-macos
	zip -m -j $(BINDIR)/$(NAME)-win64.zip $(BINDIR)/$(NAME)-win64.exe
	gzip $(BINDIR)/$(NAME)-arm

clean:
	rm $(BINDIR)/*
