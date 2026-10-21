VERSION ?= 1.0.0
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build build-all run clean tidy

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o pkghub .

build-all:
	@mkdir -p dist
	@for os in windows linux darwin; do \
	  ext=; [ "$$os" = "windows" ] && ext=.exe; \
	  echo "==> 构建 $$os/amd64"; \
	  GOOS=$$os GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/pkghub-$$os-amd64$$ext . ; \
	done

run:
	go run .

tidy:
	go mod tidy

clean:
	rm -rf pkghub pkghub.exe dist
