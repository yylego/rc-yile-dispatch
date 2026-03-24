package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/yylego/must"
	"github.com/yylego/rc-yile-dispatch/internal/model"
	"github.com/yylego/rc-yile-dispatch/internal/service"
	"github.com/yylego/rese"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	db := rese.P1(gorm.Open(sqlite.Open("dispatch.db"), &gorm.Config{}))
	defer func() { must.Done(rese.P1(db.DB()).Close()) }()

	must.Done(db.AutoMigrate(&model.Task{}))

	quit := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		close(quit)
	}()

	service.Run(db, ":8088", quit)
}
