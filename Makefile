.PHONY: check fmt-check vet test

check: fmt-check vet test

fmt-check:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "gofmt: 整形されていないファイルがある。gofmt -w で整形すること:" >&2; \
		echo "$$out" >&2; \
		exit 1; \
	fi

vet:
	go vet ./...

test:
	go test ./...
