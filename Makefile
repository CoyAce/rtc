# https://gioui.org/doc/install
NAME=RTC
BIN_DIR=bin
VERSION=$(shell cat assets/assets.go | grep 'Version =' | sed 's/.*\"\(.*\)\".*/\1/g')
GO_BUILD=CGO_ENABLED=0 go build --ldflags="-s -w" -v -x -a
GO_FILES=*.go

#PLATFORM_LIST = \
#	linux-amd64 \
#   linux-arm64

WINDOWS_ARCH_LIST = \
	windows-amd64 \
	windows-arm64

all: clean release

#linux-amd64:
#	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build --ldflags="-s -w" -v -x -a -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@ $(GO_FILES)
#
#linux-arm64:
#	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build --ldflags="-s -w" -v -x -a -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@ $(GO_FILES)

macos-amd64:
	gogio -x -work -target macos -arch amd64 -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@.app .

macos-arm64:
	gogio -x -work -target macos -arch arm64 -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@.app .

windows-amd64:
	gogio -x -work -target windows -arch amd64 -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@.exe .
	rm -f *.syso

windows-arm64:
	gogio -x -work -target windows -arch arm64 -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@.exe .
	rm -f *.syso

#ios:
#	gogio -x -work -target ios -arch arm64 -minsdk 15 -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@.ipa .

# go install gioui.org/cmd/gogio@latest
android:
	gogio -x -work -target android -arch arm64,amd64 -minsdk 22 -targetsdk 35 -version $(VERSION).1 -name RTC -appid coyace.rtc -o $(BIN_DIR)/$(NAME)-$(VERSION).apk .

gz_releases=$(addsuffix .gz, $(PLATFORM_LIST))
zip_releases=$(addsuffix .zip, $(WINDOWS_ARCH_LIST))

$(gz_releases): %.gz : %
	chmod +x $(BIN_DIR)/$(NAME)-$(VERSION)-$(basename $@)
	gzip -f -S .gz $(BIN_DIR)/$(NAME)-$(VERSION)-$(basename $@)

$(zip_releases): %.zip : %
	zip -m -j $(BIN_DIR)/$(NAME)-$(VERSION)-$(basename $@).zip $(BIN_DIR)/$(NAME)-$(VERSION)-$(basename $@).exe


release: $(gz_releases) $(zip_releases) android macos-amd64 macos-arm64

clean:
	rm -f *.syso
	rm -rf $(BIN_DIR)/*
