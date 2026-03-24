// Code generated using gormcngen. DO NOT EDIT.
// This file was auto generated via github.com/yylego/gormcngen

//go:build !gormcngen_generate

// Generated from: gormcnm_gen_test.go:25 -> model_test.TestGenerateColumns
// ========== GORMCNGEN:DO-NOT-EDIT-MARKER:END ==========

package model

import (
	"time"

	"github.com/yylego/gormcnm"
	"gorm.io/gorm"
)

func (c *Task) Columns() *TaskColumns {
	return &TaskColumns{
		// Auto-generated: column names and types mapping. DO NOT EDIT. // 自动生成：列名和类型映射。请勿编辑。
		ID:         gormcnm.Cnm(c.ID, "id"),
		CreatedAt:  gormcnm.Cnm(c.CreatedAt, "created_at"),
		UpdatedAt:  gormcnm.Cnm(c.UpdatedAt, "updated_at"),
		DeletedAt:  gormcnm.Cnm(c.DeletedAt, "deleted_at"),
		Method:     gormcnm.Cnm(c.Method, "method"),
		TargetURL:  gormcnm.Cnm(c.TargetURL, "target_url"),
		Headers:    gormcnm.Cnm(c.Headers, "headers"),
		Body:       gormcnm.Cnm(c.Body, "body"),
		Status:     gormcnm.Cnm(c.Status, "status"),
		Retries:    gormcnm.Cnm(c.Retries, "retries"),
		MaxRetries: gormcnm.Cnm(c.MaxRetries, "max_retries"),
		NextRunAt:  gormcnm.Cnm(c.NextRunAt, "next_run_at"),
		LastError:  gormcnm.Cnm(c.LastError, "last_error"),
		Callback:   gormcnm.Cnm(c.Callback, "callback"),
	}
}

type TaskColumns struct {
	// Auto-generated: embedding operation functions to make it simple to use. DO NOT EDIT. // 自动生成：嵌入操作函数便于使用。请勿编辑。
	gormcnm.ColumnOperationClass
	// Auto-generated: column names and types in database table. DO NOT EDIT. // 自动生成：数据库表的列名和类型。请勿编辑。
	ID         gormcnm.ColumnName[uint]
	CreatedAt  gormcnm.ColumnName[time.Time]
	UpdatedAt  gormcnm.ColumnName[time.Time]
	DeletedAt  gormcnm.ColumnName[gorm.DeletedAt]
	Method     gormcnm.ColumnName[string]
	TargetURL  gormcnm.ColumnName[string]
	Headers    gormcnm.ColumnName[string]
	Body       gormcnm.ColumnName[string]
	Status     gormcnm.ColumnName[TaskStatus]
	Retries    gormcnm.ColumnName[int]
	MaxRetries gormcnm.ColumnName[int]
	NextRunAt  gormcnm.ColumnName[int64]
	LastError  gormcnm.ColumnName[string]
	Callback   gormcnm.ColumnName[string]
}
