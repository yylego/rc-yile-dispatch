package service

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yylego/must"
	"github.com/yylego/rc-yile-dispatch/internal/dispatch"
	"github.com/yylego/rc-yile-dispatch/internal/handlers"
	"github.com/yylego/rc-yile-dispatch/internal/store"
	"gorm.io/gorm"
)

func Run(db *gorm.DB, address string, quit <-chan struct{}) {
	s := store.New(db)
	h := handlers.New(s)
	d := dispatch.NewDispatcher(s)

	d.Start()

	engine := gin.New()
	engine.Use(gin.Recovery())

	engine.POST("/api/dispatch", h.Submit())
	engine.GET("/api/task", h.GetTask())
	engine.GET("/api/tasks", h.ListTasks())
	engine.GET("/health", h.Health())

	log.Printf("[service] listening on %s", address)

	go func() {
		must.Done(http.ListenAndServe(address, engine))
	}()

	<-quit

	log.Println("[service] shutting down...")
	d.Stop()
	log.Println("[service] done")
}
