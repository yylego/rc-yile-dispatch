package model_test

import (
	"testing"

	"github.com/yylego/gormcngen"
	"github.com/yylego/rc-yile-dispatch/internal/model"
	"github.com/yylego/runpath/runtestpath"
)

//go:generate go test -run ^TestGenerateColumns$ -tags=gormcngen_generate
func TestGenerateColumns(t *testing.T) {
	absPath := runtestpath.SrcPath(t)
	t.Log(absPath)

	objects := []any{
		&model.Task{},
	}

	options := gormcngen.NewOptions().
		WithColumnClassExportable(true).
		WithColumnsMethodRecvName("c").
		WithColumnsCheckFieldType(true)

	cfg := gormcngen.NewConfigs(objects, options, absPath)
	cfg.Gen()
}
