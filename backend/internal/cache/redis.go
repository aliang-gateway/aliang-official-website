package cache

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	Timeout  time.Duration
}

type RedisClient struct {
	addr     string
	password string
	db       int
	timeout  time.Duration
}

func NewRedisClient(cfg RedisConfig) (*RedisClient, error) {
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		return nil, errors.New("redis addr is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &RedisClient{
		addr:     addr,
		password: strings.TrimSpace(cfg.Password),
		db:       cfg.DB,
		timeout:  timeout,
	}, nil
}

func (c *RedisClient) Get(ctx context.Context, key string) ([]byte, bool, error) {
	conn, reader, err := c.open(ctx)
	if err != nil {
		return nil, false, err
	}
	defer conn.Close()

	if err := writeRedisCommand(conn, "GET", key); err != nil {
		return nil, false, err
	}
	return readBulkString(reader)
}

func (c *RedisClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	conn, reader, err := c.open(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		seconds = 60
	}
	if err := writeRedisCommand(conn, "SETEX", key, strconv.Itoa(seconds), string(value)); err != nil {
		return err
	}
	return readSimpleOK(reader)
}

func (c *RedisClient) open(ctx context.Context) (net.Conn, *bufio.Reader, error) {
	var dialer net.Dialer
	dialCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	conn, err := dialer.DialContext(dialCtx, "tcp", c.addr)
	if err != nil {
		return nil, nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(c.timeout))

	reader := bufio.NewReader(conn)
	if c.password != "" {
		if err := writeRedisCommand(conn, "AUTH", c.password); err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
		if err := readSimpleOK(reader); err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
	}
	if c.db > 0 {
		if err := writeRedisCommand(conn, "SELECT", strconv.Itoa(c.db)); err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
		if err := readSimpleOK(reader); err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
	}
	return conn, reader, nil
}

func writeRedisCommand(conn net.Conn, parts ...string) error {
	if _, err := fmt.Fprintf(conn, "*%d\r\n", len(parts)); err != nil {
		return err
	}
	for _, part := range parts {
		if _, err := fmt.Fprintf(conn, "$%d\r\n%s\r\n", len(part), part); err != nil {
			return err
		}
	}
	return nil
}

func readSimpleOK(reader *bufio.Reader) error {
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimRight(line, "\r\n")
	if strings.HasPrefix(line, "+") {
		return nil
	}
	if strings.HasPrefix(line, "-") {
		return errors.New(strings.TrimPrefix(line, "-"))
	}
	return fmt.Errorf("unexpected redis response: %s", line)
}

func readBulkString(reader *bufio.Reader) ([]byte, bool, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, false, err
	}
	line = strings.TrimRight(line, "\r\n")
	if strings.HasPrefix(line, "-") {
		return nil, false, errors.New(strings.TrimPrefix(line, "-"))
	}
	if !strings.HasPrefix(line, "$") {
		return nil, false, fmt.Errorf("unexpected redis bulk response: %s", line)
	}
	size, err := strconv.Atoi(strings.TrimPrefix(line, "$"))
	if err != nil {
		return nil, false, err
	}
	if size < 0 {
		return nil, false, nil
	}
	buf := make([]byte, size+2)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return nil, false, err
	}
	return buf[:size], true, nil
}
