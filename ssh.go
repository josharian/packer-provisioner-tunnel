package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/mitchellh/packer/packer"
	"golang.org/x/crypto/ssh"
)

type sshServer struct {
	config   *ssh.ServerConfig
	listener net.Listener
	username string
	password string
	port     int
	comm     packer.Communicator
}

func newSSHServer() (*sshServer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, err
	}
	username := randstr(20)
	password := randstr(20)

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// Don't worry about constant time compares, etc.
			// This is a single-use username and password
			// on a randomized port, on the loopback interface
			// only, on a friendly machine.
			var err error
			if c.User() != username || string(pass) != password {
				err = errors.New("authentication failed")
			}
			return nil, err
		},
	}
	config.AddHostKey(signer)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(strings.TrimPrefix(l.Addr().String(), "127.0.0.1:"))
	if err != nil {
		return nil, err
	}

	server := sshServer{
		config:   config,
		listener: l,
		port:     port,
		username: username,
		password: password,
	}

	return &server, nil
}

func (s *sshServer) serveOne() error {
	// Accept a connection.
	c, err := s.listener.Accept()
	if err != nil {
		return err
	}
	defer c.Close()

	// Handshake.
	_, chans, reqs, err := ssh.NewServerConn(c, s.config)
	if err != nil {
		return err
	}

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if err := s.handleChannel(newChannel); err != nil {
			return err
		}
	}

	return nil
}

func (s *sshServer) handleChannel(newChannel ssh.NewChannel) error {
	if newChannel.ChannelType() != "session" {
		newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
		return nil
	}

	channel, requests, err := newChannel.Accept()
	if err != nil {
		log.Println("newChannel accept failed: ", err)
		return nil
	}

	return s.handleRequests(channel, requests)
}

type envReq struct {
	Key []byte
	Val []byte
}

type execReq struct {
	Command []byte
}

func (s *sshServer) handleRequests(channel ssh.Channel, in <-chan *ssh.Request) error {
	env := make(map[string]string)

	for req := range in {
		switch req.Type {
		default:
			log.Printf("unrecognized ssh request type=%q payload=%s wantreply=%t", req.Type, req.Payload, req.WantReply)
			req.Reply(false, nil) // unhandled; tell them so

		case "env":
			var e envReq
			if err := ssh.Unmarshal(req.Payload, &e); err != nil {
				req.Reply(false, nil)
				return err
			}
			req.Reply(true, nil)
			env[string(e.Key)] = string(e.Val)

		case "exec":
			var e execReq
			if err := ssh.Unmarshal(req.Payload, &e); err != nil {
				req.Reply(false, nil)
				return err
			}
			req.Reply(true, nil)

			var cmdbuf bytes.Buffer
			for k, v := range env {
				cmdbuf.WriteString(k)
				cmdbuf.WriteByte('=')
				cmdbuf.WriteString(v)
				cmdbuf.WriteByte(' ')
			}
			cmdbuf.Write(e.Command)

			log.Printf("Running command %q", cmdbuf.String())
			cmd := &packer.RemoteCmd{Command: cmdbuf.String()}
			cmd.Stdout = channel
			cmd.Stderr = channel.Stderr()
			var rc int
			if err := s.comm.Start(cmd); err != nil {
				rc = 255 // TODO: What is a better choice here?
			} else {
				cmd.Wait()
				rc = cmd.ExitStatus
			}
			channel.CloseWrite()
			channel.SendRequest("exit-status", false, []byte{0, 0, 0, byte(rc)})
			channel.Close()
		}
	}

	return nil
}

func randstr(n int) string {
	buf := make([]byte, n/2)
	_, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}
