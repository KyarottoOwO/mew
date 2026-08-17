package main

import (
	"net"
	"testing"
)

const pipeTestName = `\\.\pipe\discord-ipc-other`

func TestDiscordRemoveConnStaleReader(t *testing.T) {
	a := &App{discordConns: map[string]net.Conn{}}
	c1, _ := net.Pipe()
	c2, _ := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	const pipe = `\\.\pipe\discord-ipc-test`

	a.discordConns[pipe] = c1
	a.discordRemoveConn(pipe, c1)
	if _, ok := a.discordConns[pipe]; ok {
		t.Fatal("current reader should remove its connection")
	}

	a.discordConns[pipe] = c2
	a.discordRemoveConn(pipe, c1)
	if got := a.discordConns[pipe]; got != c2 {
		t.Fatal("stale reader from a previous connection evicted the new connection")
	}
}

func TestDiscordRemoveConnUnknown(t *testing.T) {
	a := &App{discordConns: map[string]net.Conn{}}
	c1, _ := net.Pipe()
	c2, _ := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	a.discordConns[pipeTestName] = c1
	a.discordRemoveConn(pipeTestName, c2)
	if got := a.discordConns[pipeTestName]; got != c1 {
		t.Fatal("unknown reader removed a connection it does not own")
	}
}
