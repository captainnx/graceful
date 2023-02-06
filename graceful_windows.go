// +build windows

package graceful

import (
	"github.com/gofiber/fiber/v2"
	"time"
)

// address defines addr as well as its network type
type address struct {
	addr    string // ip:port, unix path
	network string // tcp, unix
}

type option struct {
	watchInterval time.Duration
	stopTimeout   time.Duration
}

type Server struct {
	opt      *option
	addrs    []address
	handlers []*fiber.App
}

func NewServer(opts ...option) *Server {
	panic("platform windows unsupported")
	return nil
}

func (s *Server) Register(addr string, handler *fiber.App) {
}

func (s *Server) RegisterUnix(addr string, handler *fiber.App) {
}

func (s *Server) Run() error {
	panic("platform windows unsupported")
	return nil
}

func IsMaster() bool {
	return true
}

func IsWorker() bool {
	return false
}

func ListenAndServe(addr string, handler *fiber.App) error {
	panic("platform windows unsupported")
	return nil
}
