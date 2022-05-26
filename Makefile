PKG := github.com/guggero/wasm-demo
LDFLAGS := -s -w -buildid=

wasm:
	# This copies the latest version of `wasm_exec.js`.
	cp $(shell go env GOROOT)/misc/wasm/wasm_exec.js ./lib/wasm_exec.js
	
	# The appengine build tag is needed because of the jessevdk/go-flags library
	# that has some OS specific terminal code that doesn't compile to WASM.
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="$(LDFLAGS)" -tags="appengine" -v -o wasm-demo.wasm .

demo-server:
	go run demo-server/main.go ./ 8080

.PHONY: demo-server
