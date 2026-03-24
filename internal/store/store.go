package store

import (
	"context"
	"time"

	"github.com/yylego/gormcnm"
	"github.com/yylego/gormrepo"
	"github.com/yylego/gormrepo/gormclass"
	"github.com/yylego/rc-yile-dispatch/internal/model"
	"gorm.io/gorm"
)

type Store struct {
	db   *gorm.DB
	repo *gormrepo.Repo[model.Task, *model.TaskColumns]
}

func New(db *gorm.DB) *Store {
	return &Store{
		db:   db,
		repo: gormrepo.NewRepo(gormclass.Use(&model.Task{})),
	}
}

func (s *Store) CreateTask(ctx context.Context, task *model.Task) error {
	return s.repo.With(ctx, s.db).Create(task)
}

func (s *Store) FetchPendingTasks(ctx context.Context, limit int) ([]*model.Task, error) {
	now := time.Now().Unix()
	return s.repo.With(ctx, s.db).Find(func(db *gorm.DB, cls *model.TaskColumns) *gorm.DB {
		return db.Where(cls.Status.In([]model.TaskStatus{model.StatusPending, model.StatusFailed})).
			Where(cls.NextRunAt.Lte(now)).
			Order(cls.NextRunAt.Ob("ASC").Ox()).
			Limit(limit)
	})
}

func (s *Store) MarkRunning(ctx context.Context, id uint) error {
	return s.repo.With(ctx, s.db).UpdatesM(func(db *gorm.DB, cls *model.TaskColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(id))
	}, func(cls *model.TaskColumns) gormcnm.ColumnValueMap {
		return cls.Kw(cls.Status.Kv(model.StatusRunning))
	})
}

func (s *Store) MarkSuccess(ctx context.Context, id uint) error {
	return s.repo.With(ctx, s.db).UpdatesM(func(db *gorm.DB, cls *model.TaskColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(id))
	}, func(cls *model.TaskColumns) gormcnm.ColumnValueMap {
		return cls.Kw(cls.Status.Kv(model.StatusSuccess)).Kw(cls.LastError.Kv(""))
	})
}

func (s *Store) MarkFailed(ctx context.Context, id uint, retries int, maxRetries int, errMsg string) error {
	if retries >= maxRetries {
		return s.repo.With(ctx, s.db).UpdatesM(func(db *gorm.DB, cls *model.TaskColumns) *gorm.DB {
			return db.Where(cls.ID.Eq(id))
		}, func(cls *model.TaskColumns) gormcnm.ColumnValueMap {
			return cls.Kw(cls.Status.Kv(model.StatusDeadLine)).
				Kw(cls.Retries.Kv(retries)).
				Kw(cls.LastError.Kv(errMsg))
		})
	}

	backoff := time.Duration(1<<uint(retries)) * time.Second * 2
	nextRun := time.Now().Add(backoff).Unix()

	return s.repo.With(ctx, s.db).UpdatesM(func(db *gorm.DB, cls *model.TaskColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(id))
	}, func(cls *model.TaskColumns) gormcnm.ColumnValueMap {
		return cls.Kw(cls.Status.Kv(model.StatusFailed)).
			Kw(cls.Retries.Kv(retries)).
			Kw(cls.NextRunAt.Kv(nextRun)).
			Kw(cls.LastError.Kv(errMsg))
	})
}

func (s *Store) GetTask(ctx context.Context, id uint) (*model.Task, error) {
	return s.repo.With(ctx, s.db).First(func(db *gorm.DB, cls *model.TaskColumns) *gorm.DB {
		return db.Where(cls.ID.Eq(id))
	})
}

func (s *Store) ListTasks(ctx context.Context, status string, page, pageSize int) ([]*model.Task, int64, error) {
	return s.repo.With(ctx, s.db).FindC(func(db *gorm.DB, cls *model.TaskColumns) *gorm.DB {
		return db.Order(cls.ID.Ob("DESC").Ox()).Offset((page - 1) * pageSize).Limit(pageSize)
	}, func(db *gorm.DB, cls *model.TaskColumns) *gorm.DB {
		if status != "" {
			db = db.Where(cls.Status.Eq(model.TaskStatus(status)))
		}
		return db
	})
}
