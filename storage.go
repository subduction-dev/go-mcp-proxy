package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/client/transport"
)

type Storage struct {
	path string

	mutex sync.RWMutex
}

type ClientInfo struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
}

type StorageFormat struct {
	ClientInfo *ClientInfo      `json:"client_info"`
	Token      *transport.Token `json:"token"`
}

func NewStorage(path string) *Storage {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get home directory: %v", err)
		}
		path = filepath.Join(home, path[2:])
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("Failed to resolve path: %v", err)
	}
	return &Storage{path: absPath}
}

func (s *Storage) read() (*StorageFormat, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return nil, nil
	}

	bytes, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}

	data := &StorageFormat{}

	err = json.Unmarshal(bytes, data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *Storage) write(data *StorageFormat) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, bytes, 0600)
}

func (s *Storage) GetToken(_ context.Context) (*transport.Token, error) {
	data, err := s.read()
	if err != nil {
		return nil, err
	}

	if data == nil || data.Token == nil {
		return nil, transport.ErrNoToken
	}

	return data.Token, nil
}

func (s *Storage) SaveToken(_ context.Context, token *transport.Token) error {
	data, err := s.read()
	if err != nil {
		return err
	}

	if data == nil {
		data = &StorageFormat{}
	}

	data.Token = token

	return s.write(data)
}

func (s *Storage) GetClientInfo() (*ClientInfo, error) {
	data, err := s.read()
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	return data.ClientInfo, nil
}

func (s *Storage) SaveClientInfo(clientInfo *ClientInfo) error {
	data, err := s.read()
	if err != nil {
		return err
	}

	if data == nil {
		data = &StorageFormat{}
	}

	data.ClientInfo = clientInfo

	return s.write(data)
}
