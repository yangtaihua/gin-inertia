package inertia

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type MiddlewareFunc func(http.Handler) http.Handler

func Middleware(inertia *Inertia) gin.HandlerFunc {

	return func(c *gin.Context) {
		if c.GetHeader("X-Inertia") == "" {
			c.Next()
		}
		// In case the assets version in the X-Inertia-Version header does not match the current version
		// of assets we have on the server, return a 409 response which will cause Inertia to make a new
		// hard visit.
		if c.Request.Method == "GET" && c.GetHeader("X-Inertia-Version") != inertia.getVersion() {
			c.Writer.Header().Add("X-Inertia-Location", c.Request.RequestURI)
			c.AbortWithStatus(409)
			return
		}
		c.Next()
	}

}

type responseWriter struct {
	http.ResponseWriter
	req *http.Request
}

func (rw *responseWriter) WriteHeader(status int) {
	if status == http.StatusFound {
		switch rw.req.Method {
		case "PUT", "PATCH", "DELETE":
			rw.ResponseWriter.WriteHeader(http.StatusSeeOther)
			return
		}
	}
	rw.ResponseWriter.WriteHeader(status)
}
