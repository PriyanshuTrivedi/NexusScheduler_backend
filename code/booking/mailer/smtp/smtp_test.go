package smtp

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	s := New("smtp.example.com", 587, "user", "password")

	require.NotNil(t, s)

	sender, ok := s.(*sender)
	require.True(t, ok)
	require.NotNil(t, sender.dialer)
	require.Equal(t, "smtp.example.com", sender.dialer.Host)
	require.Equal(t, 587, sender.dialer.Port)
	require.Equal(t, "user", sender.dialer.Username)
	require.Equal(t, "password", sender.dialer.Password)
}

func TestSender_Send_ContextCancelled(t *testing.T) {
	s := New("127.0.0.1", 2525, "", "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Send(ctx, "from@example.com", "to@example.com", "Test", "<h1>Hello</h1>")

	require.ErrorIs(t, err, context.Canceled)
}

func TestSender_Send_Success(t *testing.T) {
	server, addr := startSMTPTestServer(t)
	defer server.Close()

	host, portString, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	var port int
	_, err = fmt.Sscanf(portString, "%d", &port)
	require.NoError(t, err)

	s := New(host, port, "", "").(*sender)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = s.Send(
		ctx,
		"from@example.com",
		"to@example.com",
		"Test Subject",
		"<h1>Hello</h1>",
	)

	require.NoError(t, err)
}

func TestSender_Send_DialError(t *testing.T) {
	s := New("127.0.0.1", 1, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := s.Send(
		ctx,
		"from@example.com",
		"to@example.com",
		"Test",
		"<h1>Hello</h1>",
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "smtp send")
}

func startSMTPTestServer(t *testing.T) (net.Listener, string) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go handleSMTPConnection(conn)
		}
	}()

	return listener, listener.Addr().String()
}

func handleSMTPConnection(conn net.Conn) {
	defer conn.Close()

	write := func(message string) {
		_, _ = conn.Write([]byte(message))
	}

	write("220 localhost ESMTP\r\n")

	buffer := make([]byte, 4096)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}

		command := strings.TrimSpace(string(buffer[:n]))
		commandUpper := strings.ToUpper(command)

		switch {
		case strings.HasPrefix(commandUpper, "EHLO"):
			write("250-localhost\r\n250 AUTH PLAIN LOGIN\r\n")

		case strings.HasPrefix(commandUpper, "HELO"):
			write("250 localhost\r\n")

		case strings.HasPrefix(commandUpper, "AUTH"):
			write("235 Authentication successful\r\n")

		case strings.HasPrefix(commandUpper, "MAIL FROM"):
			write("250 OK\r\n")

		case strings.HasPrefix(commandUpper, "RCPT TO"):
			write("250 OK\r\n")

		case strings.HasPrefix(commandUpper, "DATA"):
			write("354 End data with <CR><LF>.<CR><LF>\r\n")

			for {
				n, err = conn.Read(buffer)
				if err != nil {
					return
				}

				data := string(buffer[:n])
				if strings.Contains(data, "\r\n.\r\n") || strings.HasSuffix(data, "\n.\n") {
					break
				}
			}

			write("250 OK\r\n")

		case commandUpper == "QUIT":
			write("221 Bye\r\n")
			return

		default:
			write("250 OK\r\n")
		}
	}
}
