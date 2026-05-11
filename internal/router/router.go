package router

import (
	"github.com/gin-gonic/gin"

	"markdown-notes/internal/handler"
	"markdown-notes/internal/repository"
	"markdown-notes/internal/service"
	"markdown-notes/pkg/database"
)

func New(db *database.Postgres) *gin.Engine {
	r := gin.Default()

	// CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Setup dependencies
	noteRepo := repository.NewNoteRepository(db.Pool)
	noteService := service.NewNoteService(noteRepo)
	noteHandler := handler.NewNoteHandler(noteService)

	// Routes
	api := r.Group("/api/v1")
	{
		notes := api.Group("/notes")
		{
			notes.POST("", noteHandler.Create)
			notes.GET("", noteHandler.GetAll)
			notes.GET("/:id", noteHandler.GetByID)
			notes.PUT("/:id", noteHandler.Update)
			notes.DELETE("/:id", noteHandler.Delete)
		}
	}

	return r
}
