package routes

import (
	"backendAPI/db"
	"backendAPI/models"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

var getSlashings = db.GetSlashings
var getValidatorSetChanges = db.GetValidatorSetChanges

func activityQuery(c *gin.Context, changes bool) (models.ValidatorActivityQuery, error) {
	q := models.ValidatorActivityQuery{Page: 1, Limit: 20}
	for _, param := range []struct {
		name   string
		target *int
		max    int64
	}{
		{"page", &q.Page, 1000000}, {"limit", &q.Limit, 100},
	} {
		if raw, exists := c.GetQuery(param.name); exists {
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || n < 1 || n > param.max {
				return q, fmt.Errorf("invalid %s", param.name)
			}
			*param.target = int(n)
		}
	}
	if raw, exists := c.GetQuery("validator"); exists {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return q, fmt.Errorf("invalid validator index")
		}
		q.ValidatorIndex = strconv.FormatUint(n, 10)
	}
	if changes {
		q.Kind = c.Query("kind")
		switch q.Kind {
		case "", "activated", "exited", "slashed", "ejected":
		default:
			return q, fmt.Errorf("invalid change kind")
		}
		for _, param := range []struct {
			name   string
			target **int64
		}{
			{"fromEpoch", &q.FromEpoch}, {"toEpoch", &q.ToEpoch},
		} {
			if raw, exists := c.GetQuery(param.name); exists {
				n, err := strconv.ParseInt(raw, 10, 64)
				if err != nil || n < 0 {
					return q, fmt.Errorf("invalid %s", param.name)
				}
				*param.target = &n
			}
		}
		if q.FromEpoch != nil && q.ToEpoch != nil && *q.FromEpoch > *q.ToEpoch {
			return q, fmt.Errorf("fromEpoch must be at most toEpoch")
		}
	}
	return q, nil
}

func handleSlashings(c *gin.Context) {
	q, err := activityQuery(c, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := getSlashings(c.Request.Context(), q)
	if err != nil {
		log.Printf("error fetching slashings: %v", err)
		respondInternal(c)
		return
	}
	if result.Events == nil {
		result.Events = make([]models.SlashingEvent, 0)
	}
	c.JSON(http.StatusOK, result)
}

func handleValidatorSetChanges(c *gin.Context) {
	q, err := activityQuery(c, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := getValidatorSetChanges(c.Request.Context(), q)
	if err != nil {
		log.Printf("error fetching validator set changes: %v", err)
		respondInternal(c)
		return
	}
	if result.Changes == nil {
		result.Changes = make([]models.ValidatorSetChange, 0)
	}
	c.JSON(http.StatusOK, result)
}
