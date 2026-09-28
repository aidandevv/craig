package main

import (
	"net/http"
	"testing"
)

func TestDaemonHTTPServerSetsBoundedConnectionLimits(t *testing.T) {
	server := newDaemonHTTPServer(http.NotFoundHandler())
	if server.ReadHeaderTimeout != daemonReadHeaderTimeout || server.ReadTimeout != daemonReadTimeout || server.WriteTimeout != daemonWriteTimeout || server.IdleTimeout != daemonIdleTimeout || server.MaxHeaderBytes != daemonMaxHeaderSize {
		t.Errorf("daemon server limits = %+v", server)
	}
}
