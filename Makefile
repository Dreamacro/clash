NAME=clash
BINDIR=bin
VERSION=$(shell git describe --tags || echo "unknown version")
BUILDTIME=$(shell date -u)
GOBUILD=CGO_ENABLED=0 go build -ldflags '-X "github.com/Dreamacro/clash/constant.Version=$(VERSION)" \
		-X "github.com/Dreamacro/clash/constant.BuildTime=$(BUILDTIME)" \
		-w -s'
MOBILE_GOBUILD=CGO_ENABLED=1 go build -ldflags '-X "github.com/Dreamacro/clash/constant.Version=$(VERSION)" \
		-X "github.com/Dreamacro/clash/constant.BuildTime=$(BUILDTIME)" \
		-w -s'

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
	GOARCH=386 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

windows-amd64:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

android-armeabi-v7a:
ifndef ANDROID_NDK_HOST
	@echo "ANDROID_NDK_HOST is undefined, use default linux-x86_64"
	$(eval ANDROID_NDK_HOST := linux-x86_64)
endif
ifndef ANDROID_NDK
	$(error ANDROID_NDK is undefined)
endif
ifndef ANDROID_API
	@echo "ANDROID_API is undefined, use default 21"
	$(eval ANDROID_API := 21)
endif
	$(eval ANDROID_CC := $(ANDROID_NDK)/toolchains/llvm/prebuilt/$(ANDROID_NDK_HOST)/bin/armv7a-linux-androideabi$(ANDROID_API)-clang)
	$(eval ABDROID_CXX := $(ANDROID_NDK)/toolchains/llvm/prebuilt/$(ANDROID_NDK_HOST)/bin/armv7a-linux-androideabi$(ANDROID_API)-clang++)
	$(eval ANDROID_LD := $(ANDROID_NDK)/toolchains/llvm/prebuilt/$(ANDROID_NDK_HOST)/bin/arm-linux-androideabi-ld)
	GOARCH=arm GOARM=7 GOOS=android CXX=$(ABDROID_CXX) CC=$(ANDROID_CC) LD=$(ANDROID_LD)  $(MOBILE_GOBUILD) -o $(BINDIR)/$(NAME)-$@

android-arm64-v8a:
ifndef ANDROID_NDK_HOST
	@echo "ANDROID_NDK_HOST is undefined, use default linux-x86_64"
	$(eval ANDROID_NDK_HOST := linux-x86_64)
endif
ifndef ANDROID_NDK
	$(error ANDROID_NDK is undefined)
endif
ifndef ANDROID_API
	@echo "ANDROID_API is undefined, use default 21"
	$(eval ANDROID_API := 21)
endif
	$(eval ANDROID_CC := $(ANDROID_NDK)/toolchains/llvm/prebuilt/$(ANDROID_NDK_HOST)/bin/aarch64-linux-android$(ANDROID_API)-clang)
	$(eval ABDROID_CXX := $(ANDROID_NDK)/toolchains/llvm/prebuilt/$(ANDROID_NDK_HOST)/bin/aarch64-linux-android$(ANDROID_API)-clang++)
	$(eval ANDROID_LD := $(ANDROID_NDK)/toolchains/llvm/prebuilt/$(ANDROID_NDK_HOST)/bin/aarch64-linux-android-ld)
	GOARCH=arm64 GOOS=android CXX=$(ABDROID_CXX) CC=$(ANDROID_CC) LD=$(ANDROID_LD)  $(MOBILE_GOBUILD) -o $(BINDIR)/$(NAME)-$@

gz_releases=$(addsuffix .gz, $(PLATFORM_LIST))
zip_releases=$(addsuffix .zip, $(WINDOWS_ARCH_LIST))

$(gz_releases): %.gz : %
	chmod +x $(BINDIR)/$(NAME)-$(basename $@)
	gzip -f -S -$(VERSION).gz $(BINDIR)/$(NAME)-$(basename $@)

$(zip_releases): %.zip : %
	zip -m -j $(BINDIR)/$(NAME)-$(basename $@)-$(VERSION).zip $(BINDIR)/$(NAME)-$(basename $@).exe

all-arch: $(PLATFORM_LIST) $(WINDOWS_ARCH_LIST)

releases: $(gz_releases) $(zip_releases)
clean:
	rm $(BINDIR)/*
