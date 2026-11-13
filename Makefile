TESTING_FW  = /Library/Developer/CommandLineTools/Library/Developer/Frameworks
TESTING_LIB = /Library/Developer/CommandLineTools/Library/Developer/usr/lib
TEST_FLAGS  = -Xswiftc -F -Xswiftc $(TESTING_FW) -Xlinker -F -Xlinker $(TESTING_FW) -Xlinker -rpath -Xlinker $(TESTING_FW) -Xlinker -rpath -Xlinker $(TESTING_LIB)

APP_DIR = build/Yakumo.app
INSTALL_DIR = $(HOME)/Applications

.PHONY: test app install clean

test:
	swift test --enable-code-coverage $(TEST_FLAGS)

app:
	swift build -c release
	rm -rf $(APP_DIR)
	mkdir -p $(APP_DIR)/Contents/MacOS
	cp "$$(swift build -c release --show-bin-path)/Yakumo" $(APP_DIR)/Contents/MacOS/Yakumo
	cp Resources/Info.plist $(APP_DIR)/Contents/Info.plist
	codesign --force --sign - $(APP_DIR)

install: app
	mkdir -p $(INSTALL_DIR)
	rm -rf $(INSTALL_DIR)/Yakumo.app
	cp -R $(APP_DIR) $(INSTALL_DIR)/Yakumo.app

clean:
	rm -rf build .build
