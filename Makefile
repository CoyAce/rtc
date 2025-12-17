# https://gioui.org/doc/install

NAME=coyace.rtc
BIN_DIR=bin
VERSION=$(shell cat assets/assets.go | grep 'Version =' | sed 's/.*\"\(.*\)\".*/\1/g')
GO_BUILD=CGO_ENABLED=0 go build --ldflags="-s -w" -v -x -a
GO_FILES=*.go

PLATFORM_LIST = \
	linux-amd64 \

WINDOWS_ARCH_LIST = \
	windows-amd64 \
	windows-arm64

linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build --ldflags="-s -w" -v -x -a -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@ $(GO_FILES)

linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build --ldflags="-s -w" -v -x -a -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@ $(GO_FILES)

darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@ $(GO_FILES)

darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GO_BUILD) -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@ $(GO_FILES)

# https://github.com/tc-hib/go-winres
windows-amd64:
	GOOS=windows GOARCH=amd64 go-winres make --in winres/winres.json --out winres/rsrc
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -H windowsgui" -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@.exe $(GO_FILES)

windows-arm64:
	GOOS=windows GOARCH=arm64 go-winres make --in winres/winres.json --out winres/rsrc
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w -H windowsgui" -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@.exe $(GO_FILES)

# go install gioui.org/cmd/gogio@latest
android:
	#gogio -x -work -target android -minsdk 22 -targetsdk 36 -version $(VERSION).11 -name Rtc -signkey build/sign.keystore -signpass android -appid coyace.rtc -o $(BINDIR)/$(NAME)-$(VERSION).aab .
	gogio -x -work -target android -arch amd64 -minsdk 22 -targetsdk 36 -version $(VERSION).1 -name Rtc -appid coyace.rtc -o $(NAME)-$(VERSION).apk .

gz_releases=$(addsuffix .gz, $(PLATFORM_LIST))
zip_releases=$(addsuffix .zip, $(WINDOWS_ARCH_LIST))

$(gz_releases): %.gz : %
	chmod +x $(BIN_DIR)/$(NAME)-$(VERSION)-$(basename $@)
	gzip -f -S .gz $(BIN_DIR)/$(NAME)-$(VERSION)-$(basename $@)

$(zip_releases): %.zip : %
	zip -m -j $(BIN_DIR)/$(NAME)-$(VERSION)-$(basename $@).zip $(BIN_DIR)/$(NAME)-$(VERSION)-$(basename $@).exe

releases: $(gz_releases) $(zip_releases) android

clean:
	rm *.syso -f
	rm $(BIN_DIR)/* -rf
