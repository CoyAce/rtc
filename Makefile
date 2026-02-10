# https://gioui.org/doc/install
NAME=◯
BIN_DIR=bin
VERSION=$(shell cat assets/assets.go | grep 'Version =' | sed 's/.*\"\(.*\)\".*/\1/g')
LDFLAGS='-extldflags "-nostdlib++ -Wl,-Bstatic -lc++ -Wl,-Bdynamic"'

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
	mkdir -p $(BIN_DIR)/$(VERSION)/$@
	gogio -x -work -target macos -arch amd64 -o $(BIN_DIR)/$(VERSION)/$@/$(NAME).app .

macos-arm64:
	mkdir -p $(BIN_DIR)/$(VERSION)/$@
	gogio -x -work -target macos -arch arm64 -o $(BIN_DIR)/$(VERSION)/$@/$(NAME).app .

windows-amd64:
	mkdir -p $(BIN_DIR)/$(VERSION)/$@
	gogio -x -work -target windows -arch amd64 -o $(BIN_DIR)/$(VERSION)/$@/$(NAME).exe .
	rm -f *.syso

windows-arm64:
	mkdir -p $(BIN_DIR)/$(VERSION)/$@
	gogio -x -work -target windows -arch arm64 -o $(BIN_DIR)/$(VERSION)/$@/$(NAME).exe .
	rm -f *.syso

ios:
	gogio -x -work -target ios -appid coyace.rtc -arch arm64 -minsdk 15 -tags=timetzdata -o $(BIN_DIR)/$(NAME)-$(VERSION)-$@.ipa .

# go install gioui.org/cmd/gogio@latest
android:
	gogio -x -work -target android -arch arm64,amd64 -ldflags ${LDFLAGS} -version $(VERSION).3 -name $(NAME) -appid coyace.rtc -o $(BIN_DIR)/$(NAME)-$(VERSION).apk .

gz_releases=$(addsuffix .gz, $(PLATFORM_LIST))
zip_releases=$(addsuffix .zip, $(WINDOWS_ARCH_LIST))

$(gz_releases): %.gz : %
	chmod +x $(BIN_DIR)/$(NAME)-$(VERSION)-$(basename $@)
	gzip -f -S .gz $(BIN_DIR)/$(NAME)-$(VERSION)-$(basename $@)

$(zip_releases): %.zip : %
	zip -m -j $(BIN_DIR)/$(VERSION)/$(NAME)-$(VERSION)-$(basename $@).zip $(BIN_DIR)/$(VERSION)/$(basename $@)/$(NAME).exe


release: $(gz_releases) $(zip_releases) android macos-amd64 macos-arm64

clean:
	rm -f *.syso
	rm -rf $(BIN_DIR)/*
