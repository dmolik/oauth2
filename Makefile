
BIN := oauth2

SRC := $(wildcard *.go) go.mod go.sum

all: $(BIN)

$(BIN): $(SRC)
	go build -v -o $@
