package routes

import (
	"backendAPI/db"
	"backendAPI/models"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// handleEpochs serves GET /epochs, the paginated epochs listing.
func handleEpochs(c *gin.Context) {
	page, limit := getPaginationParams(c, 1, 15)

	result, err := db.GetEpochs(page, limit)
	if err != nil {
		log.Printf("error fetching epochs: %v", err)
		respondInternal(c)
		return
	}

	if result.Epochs == nil {
		result.Epochs = make([]models.EpochListItem, 0)
	}

	c.JSON(http.StatusOK, result)
}

// handleValidators serves GET /validators.
func handleValidators(c *gin.Context) {
	pageToken := c.Query("page_token")
	// ReturnValidators fetches the full validator set (one doc per
	// validator) and sums TotalStaked, an expensive unsegmented read.
	// The set only shifts at epoch boundaries (~minutes), so a 30s TTL
	// caches the full-collection fetch instead of running it per request.
	// Keyed by page_token so future server-side pagination stays correct.
	key := "validators:" + pageToken
	v, err := routeCache.GetOrCompute(key, 30*time.Second, func() (interface{}, error) {
		return db.ReturnValidators(pageToken)
	})
	if err != nil {
		log.Printf("error fetching validators: %v", err)
		respondInternal(c)
		return
	}

	c.JSON(http.StatusOK, v)
}

// handleEpochByID serves GET /epoch/:id, the epoch detail endpoint.
func handleEpochByID(c *gin.Context) {
	epochId := c.Param("id")
	epochDetail, err := db.GetEpochDetail(epochId)
	if err != nil {
		if strings.Contains(err.Error(), "not yet occurred") || strings.Contains(err.Error(), "invalid epoch") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		log.Printf("error fetching epoch detail %s: %v", epochId, err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, epochDetail)
}

// handleEpoch serves GET /epoch, the current epoch information.
func handleEpoch(c *gin.Context) {
	epochInfo, err := db.GetEpochInfo()
	if err != nil {
		log.Printf("error fetching epoch info: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, epochInfo)
}

// handleValidatorsHistory serves GET /validators/history, the validator
// history for charts.
func handleValidatorsHistory(c *gin.Context) {
	_, limit := getPaginationParams(c, 1, 100)

	history, err := db.GetValidatorHistory(limit)
	if err != nil {
		log.Printf("error fetching validator history: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, history)
}

// handleValidatorsStats serves GET /validators/stats.
func handleValidatorsStats(c *gin.Context) {
	stats, err := db.GetValidatorStats()
	if err != nil {
		log.Printf("error fetching validator stats: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, stats)
}

// handleValidator serves GET /validator/:id, individual validator details.
func handleValidator(c *gin.Context) {
	id := c.Param("id")
	if !isValidValidatorID(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid validator id; expected a decimal index or hex public key"})
		return
	}
	validator, err := db.GetValidatorByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("Validator not found: %v", err),
		})
		return
	}
	c.JSON(http.StatusOK, validator)
}
