package routes

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"backendAPI/db"
	"backendAPI/middleware"
	"backendAPI/models"
	"backendAPI/verification"

	"github.com/gin-gonic/gin"
)

// RegisterVerificationRoutes wires the three contract-verification
// endpoints onto the supplied router. Always registered, when the
// verifier isn't configured, individual handlers return 503 so the rest
// of the backend keeps working.
func RegisterVerificationRoutes(router *gin.Engine) {
	// Per-IP rate limit: 5 in-flight verifications per minute is plenty
	// for human-driven submissions and trivially budgets the runner.
	verifyLimiter := middleware.PerIPRateLimit(5, 5, time.Minute)

	router.GET("/contract/compiler-info", func(c *gin.Context) {
		v := verification.Default()
		if v == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "contract verification not configured"})
			return
		}
		c.JSON(http.StatusOK, v.Registry.Info())
	})

	// Maximum body size for a verify submission. The compiler also has
	// VERIFIER_SOURCE_MAX_BYTES (default 256 KiB) on the marshalled
	// standard-JSON payload, but that fires AFTER ShouldBindJSON has
	// already buffered the request. Cap the raw stream too so a malicious
	// gigantic body can't OOM the process during JSON binding.
	const verifyMaxRequestBytes int64 = 1 << 20 // 1 MiB

	router.POST("/contract/verify", verifyLimiter, func(c *gin.Context) {
		v := verification.Default()
		if v == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "contract verification not configured"})
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, verifyMaxRequestBytes)

		var req verification.VerifyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
			return
		}
		if !isValidAddress(req.Address) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Q-prefixed address"})
			return
		}
		if req.ContractName == "" || req.SourceCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "contractName and sourceCode are required"})
			return
		}
		// Resolve the requested build (empty selects the registry default).
		// Pin the request to the resolved build id so the stored payload and
		// the async compile can't drift from what we validated here.
		comp, ok := v.Registry.Resolve(req.CompilerVersion)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":           "unsupported compilerVersion, see /contract/compiler-info",
				"requestedBuild":  req.CompilerVersion,
				"supportedBuilds": v.Registry.SupportedBuildIDs(),
			})
			return
		}
		req.CompilerVersion = comp.BuildID

		// Idempotency: if the contract is already verified, return success
		// without spawning another compile.
		already, err := verification.AlreadyVerified(req.Address)
		if err == nil && already {
			c.JSON(http.StatusOK, gin.H{
				"alreadyVerified": true,
				"address":         req.Address,
			})
			return
		}

		// Reject duplicate in-flight jobs for the same address.
		pending, err := verification.HasPendingJob(req.Address)
		if err == nil && pending {
			c.JSON(http.StatusConflict, gin.H{"error": "a verification job is already in flight for this address"})
			return
		}

		jobID := newJobID()
		job := models.ContractVerificationJob{
			JobID:   jobID,
			Address: req.Address,
			Status:  models.VerificationJobPending,
			Payload: models.VerificationJobPayload{
				SourceCode:           req.SourceCode,
				ContractName:         req.ContractName,
				CompilerVersion:      comp.BuildID,
				OptimizationEnabled:  req.OptimizerEnabled,
				OptimizationRuns:     req.OptimizerRuns,
				EvmVersion:           req.EvmVersion,
				ConstructorArguments: req.ConstructorArguments,
				Libraries:            req.Libraries,
				License:              req.License,
				VerificationMethod:   "full-source",
			},
		}
		if err := db.CreateVerificationJob(job); err != nil {
			log.Printf("create verification job %s: %v", jobID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		v.RunAsync(jobID, req)

		c.JSON(http.StatusAccepted, verification.VerifyEnqueueResponse{
			JobID:   jobID,
			Status:  string(models.VerificationJobPending),
			Address: req.Address,
		})
	})

	router.GET("/contract/verify/:jobId", func(c *gin.Context) {
		jobID := c.Param("jobId")
		job, err := db.GetVerificationJob(jobID)
		if err != nil {
			log.Printf("lookup verification job %s: %v", jobID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if job == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}
		c.JSON(http.StatusOK, job)
	})
}

// newJobID returns a short opaque token (16 hex chars). Crypto random is
// overkill for an externally-opaque handle but it's free and keeps callers
// from guessing other users' jobs. A rand.Read failure on Linux/macOS
// means /dev/urandom is unreadable, the process is unsalvageable, so
// panic rather than continue with a degenerate ID.
func newJobID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic("verification: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(buf[:])
}

// isValidAddress is a permissive Q-prefix check. The full validation
// happens at the storage layer (normalizeAddress canonicalises case).
func isValidAddress(a string) bool {
	if !strings.HasPrefix(a, "Q") {
		return false
	}
	if len(a) != 41 {
		return false
	}
	for _, r := range a[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
