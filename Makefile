all: make-wire tree

clean:
	rm -f make-wire tree

make-wire:
	go build ./cmd/make-wire

tree:
	go build ./cmd/tree

.PHONY: clean
