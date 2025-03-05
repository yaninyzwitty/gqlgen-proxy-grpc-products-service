package pkg

import (
	"io"
	"log/slog"

	"gopkg.in/yaml.v3"
)

type Config struct {
	GrpcServer    GrpcServer    `yaml:"grpc_server"`
	GraphqlServer GraphqlServer `yaml:"graphql_server"`
}

type GrpcServer struct {
	Port string `yaml:"port"`
}

type GraphqlServer struct {
	Port string `yaml:"port"`
}

func (c *Config) LoadConfig(file io.Reader) error {
	data, err := io.ReadAll(file)
	if err != nil {
		slog.Error("failed to read file", "error", err)
		return err
	}
	err = yaml.Unmarshal(data, c)
	if err != nil {
		slog.Error("failed to unmarshal yaml", "error", err)
		return err
	}
	return nil
}
