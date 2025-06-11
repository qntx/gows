package gows

import (
	"context"
	"io"
)

type MessageType int

const (
	// MessageText is for UTF-8 encoded text messages like JSON.
	MessageText MessageType = iota + 1
	// MessageBinary is for binary messages like protobufs.
	MessageBinary
)

type Client interface {
	Connect(ctx context.Context) error
	Close() error

	Read(ctx context.Context, v any) (MessageType, []byte, error)
	Write(ctx context.Context, typ MessageType, p []byte) error

	Reader(ctx context.Context) (MessageType, io.Reader, error)
	Writer(ctx context.Context, typ MessageType) (io.WriteCloser, error)
}
