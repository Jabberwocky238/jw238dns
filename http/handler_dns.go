package http

import (
	"errors"

	"jabberwocky238/jw238dns/storage"
	"jabberwocky238/jw238dns/types"

	"github.com/gin-gonic/gin"
)

// DNSHandler handles DNS record management endpoints.
type DNSHandler struct {
	storage storage.CoreStorage
}

// NewDNSHandler creates a new DNSHandler with the given storage backend.
func NewDNSHandler(store storage.CoreStorage) *DNSHandler {
	return &DNSHandler{storage: store}
}

// AddRecord handles POST /dns/add.
func (h *DNSHandler) AddRecord(c *gin.Context) {
	var req AddRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, err.Error())
		return
	}

	if !req.Type.IsValid() {
		Fail(c, 400, "invalid record type")
		return
	}

	if req.TTL == 0 {
		req.TTL = 300
	}

	record := &types.DNSRecord{
		Name:  req.Domain,
		Type:  req.Type,
		TTL:   req.TTL,
		Value: req.Value,
	}

	if err := h.storage.Create(c.Request.Context(), record); err != nil {
		if errors.Is(err, types.ErrRecordExists) {
			Fail(c, 409, "record already exists")
			return
		}
		Fail(c, 500, err.Error())
		return
	}

	OK(c, record)
}

// DeleteRecord handles POST /dns/delete.
func (h *DNSHandler) DeleteRecord(c *gin.Context) {
	var req DeleteRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, err.Error())
		return
	}

	if !req.Type.IsValid() {
		Fail(c, 400, "invalid record type")
		return
	}

	if err := h.storage.Delete(c.Request.Context(), req.Domain, req.Type); err != nil {
		if errors.Is(err, types.ErrRecordNotFound) {
			Fail(c, 404, "record not found")
			return
		}
		Fail(c, 500, err.Error())
		return
	}

	OK(c, nil)
}

// UpdateRecord handles POST /dns/update.
func (h *DNSHandler) UpdateRecord(c *gin.Context) {
	var req UpdateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, err.Error())
		return
	}

	if !req.Type.IsValid() {
		Fail(c, 400, "invalid record type")
		return
	}

	if req.TTL == 0 {
		req.TTL = 300
	}

	record := &types.DNSRecord{
		Name:  req.Domain,
		Type:  req.Type,
		TTL:   req.TTL,
		Value: req.Value,
	}

	if err := h.storage.Update(c.Request.Context(), record); err != nil {
		if errors.Is(err, types.ErrRecordNotFound) {
			Fail(c, 404, "record not found")
			return
		}
		Fail(c, 500, err.Error())
		return
	}

	OK(c, record)
}

// ListRecords handles GET /dns/list.
func (h *DNSHandler) ListRecords(c *gin.Context) {
	records, err := h.storage.List(c.Request.Context())
	if err != nil {
		Fail(c, 500, err.Error())
		return
	}

	// Apply optional filters from query params.
	domain := c.Query("domain")
	recordType := c.Query("type")

	var filtered []*types.DNSRecord
	for _, r := range records {
		if domain != "" && r.Name != domain {
			continue
		}
		if recordType != "" && string(r.Type) != recordType {
			continue
		}
		filtered = append(filtered, r)
	}

	OK(c, filtered)
}

// GetRecord handles GET /dns/get.
func (h *DNSHandler) GetRecord(c *gin.Context) {
	domain := c.Query("domain")
	recordType := c.Query("type")

	if domain == "" || recordType == "" {
		Fail(c, 400, "domain and type query parameters are required")
		return
	}

	rt := types.RecordType(recordType)
	if !rt.IsValid() {
		Fail(c, 400, "invalid record type")
		return
	}

	records, err := h.storage.Get(c.Request.Context(), domain, rt)
	if err != nil {
		if errors.Is(err, types.ErrRecordNotFound) {
			Fail(c, 404, "record not found")
			return
		}
		Fail(c, 500, err.Error())
		return
	}

	OK(c, records)
}
