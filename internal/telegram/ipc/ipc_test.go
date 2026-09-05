package ipc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMessageVersionOK(t *testing.T) {
	if !(Message{ProtocolVersion: ProtocolVersion}).VersionOK() {
		t.Fatal("current version should be OK")
	}
	if (Message{ProtocolVersion: 0}).VersionOK() {
		t.Fatal("version 0 should be incompatible")
	}
}

func TestConnRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sock")
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	serverDone := make(chan Message, 1)
	serverErr := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer c.Close()
		sc := NewConn(c)
		m, err := sc.Recv()
		if err != nil {
			serverErr <- err
			return
		}
		serverDone <- m
	}()

	conn, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	cc := NewConn(conn)
	if err := cc.Send(Message{Type: TypeRegister, ProjectAbbrev: "myproj"}); err != nil {
		t.Fatal(err)
	}

	select {
	case m := <-serverDone:
		if m.Type != TypeRegister {
			t.Fatalf("type=%q", m.Type)
		}
		if m.ProjectAbbrev != "myproj" {
			t.Fatalf("abbrev=%q", m.ProjectAbbrev)
		}
		if !m.VersionOK() {
			t.Fatalf("version not populated: %d", m.ProtocolVersion)
		}
	case err := <-serverErr:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for server")
	}
	_ = cc.Close()
}

func TestListenSocketPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "perm.sock")
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("socket perm=%o want 600", perm)
	}
}

func TestListenDoesNotStealLiveSocket(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "live.sock")
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if _, err := Listen(path); err == nil {
		t.Fatal("second Listen must not steal a live socket")
	}
	conn, err := Dial(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
}

func TestListenReplacesStaleSocket(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stale.sock")
	if err := os.WriteFile(path, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()
}

func TestNewMessageIDUnique(t *testing.T) {
	a := NewMessageID()
	b := NewMessageID()
	if a == b {
		t.Fatal("expected distinct message ids")
	}
	if a == "" || b == "" {
		t.Fatal("empty message id")
	}
}
