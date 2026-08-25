package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// internalError responds with a fixed, generic message on every 500 —
// never err.Error() directly, which can leak raw internal detail (a SQL
// error, a stack-adjacent Go error string, ...) straight to the client.
// The real error is logged server-side instead. 4xx responses elsewhere in
// this package are unaffected — those are already meaningful, safe
// business-level messages (e.g. "the default group cannot be paused"),
// not internal failures.
func internalError(c *gin.Context, err error) {
	log.Printf("internal error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong, please try again"})
}
